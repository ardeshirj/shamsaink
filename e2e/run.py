#!/usr/bin/env python3
"""End-to-end tests on a device, run from the Mac (docs/roadmap.md, "End-to-end tests"): a scenario's steps played on
the device over SSH, its touches written into the touch screen as the kernel's input events (the reader reads them as
a finger's), its log watched for what each step should do, and the screen copied at each snap, compared with the
references kept by an earlier run (byte for byte, the clock's strip at the top aside).

    e2e/run.py [--save] DEVICE SCENARIO          (or: make e2e SCENARIO=e2e/kobo-basics.txt)

DEVICE: kobo (the Kindle to come: its book is opened through KOReader's plugin). SCENARIO: a file of steps, one a
line (# starts a comment):

    open                    the book opened as NickelMenu does (launch.sh: the last one read); waits for its page
    tap X Y                 a finger down and up at (X, Y), the screen's pixels
    swipe X Y X2 Y2         a finger moved from (X, Y) to (X2, Y2)
    hold X Y [to X2 Y2]     a finger held still (a hold), moved to (X2, Y2) if given, and lifted
    expect REGEX            waits (10 s at most) for a line of the reader's log, after the last one expected (REGEX:
                            the rest of the line)
    snap NAME               the screen copied: e2e/out/SCENARIO/NAME.pgm, compared with refs/NAME.pgm
    wait SECONDS
    close                   the menu opened and the book left; waits for Nickel to be started again

--save: the snaps kept as the references (e2e/out/SCENARIO/refs/). Never committed: pages drawn from books.
"""
import os
import re
import struct
import subprocess
import sys
import time

DEVICES = {
    "kobo": dict(
        ssh="kobo",
        dir="/mnt/onboard/.adds/shamsaink",
        log="/mnt/onboard/.adds/shamsaink/logs/reader.log",
        touch="/dev/input/event1",
        w=1264, h=1680, stride=5056, bytes=4,
        # the touch screen is turned: screen (x, y) is touch (1680 - y, x) (reader/device/screen.go, touchTurn)
        turn=lambda x, y: (1680 - y, x),
        # the menu's place on the page and its button to leave the book (reader/ui/templates/menu.html)
        menu=(632, 80), leave=(206, 91),
        top=60,  # rows of the clock's strip, left out of the comparison
    ),
}

EV_SYN, EV_ABS = 0, 3
SLOT, POS_X, POS_Y, TRACKING = 0x2F, 0x35, 0x36, 0x39


def event(t, code, value):
    # struct input_event on 32-bit ARM: a timeval (8 bytes, left 0: the reader does not read it), type, code, value
    return struct.pack("<IIHHi", 0, 0, t, code, value)


class Device:
    def __init__(self, name):
        self.__dict__.update(DEVICES[name])
        self.mark = self.lines()  # the log's lines before the run

    def sh(self, cmd, data=None):
        r = subprocess.run(["ssh", "-o", "ConnectTimeout=10", self.ssh, cmd], input=data, capture_output=True)
        if r.returncode != 0 and data is None and not cmd.startswith("cd "):
            raise RuntimeError(f"ssh {self.ssh} {cmd!r}: {r.stderr.decode().strip()}")
        return r.stdout

    def lines(self):
        return int(self.sh(f"wc -l < {self.log}").decode().split()[0])

    # touches: each part of a gesture written as one ssh call, so a hold's timing is the Mac's
    def write(self, b):
        self.sh(f"cat > {self.touch}", data=b)

    def at(self, x, y):
        tx, ty = self.turn(x, y)
        return event(EV_ABS, POS_X, tx) + event(EV_ABS, POS_Y, ty) + event(EV_SYN, 0, 0)

    def down(self, x, y):
        return event(EV_ABS, SLOT, 0) + event(EV_ABS, TRACKING, 7) + self.at(x, y)

    def up(self):
        return event(EV_ABS, TRACKING, -1) + event(EV_SYN, 0, 0)

    def tap(self, x, y):
        self.write(self.down(x, y) + self.up())

    def swipe(self, x, y, x2, y2):
        b = self.down(x, y)
        for i in range(1, 6):
            b += self.at(x + (x2 - x) * i // 5, y + (y2 - y) * i // 5)
        self.write(b + self.up())

    def hold(self, x, y, to=None):
        self.write(self.down(x, y))
        time.sleep(1.2)  # held still past the reader's 800 ms (device/touch.go, holdTime)
        if to:
            for i in range(1, 4):
                self.write(self.at(x + (to[0] - x) * i // 3, y + (to[1] - y) * i // 3))
                time.sleep(0.3)
        self.write(self.up())

    def expect(self, pattern, seconds=10.0):
        end = time.time() + seconds
        while True:
            new = self.sh(f"tail -n +{self.mark + 1} {self.log}").decode().splitlines()
            for i, line in enumerate(new):
                if re.search(pattern, line):
                    self.mark += i + 1
                    return line
            if time.time() > end:
                raise AssertionError(f"not in the log after {seconds:g} s: {pattern!r}; new lines: {new[-5:]}")
            time.sleep(0.3)

    def screen(self):
        raw = self.sh("cat /dev/fb0")
        if self.bytes == 1:
            return b"".join(raw[y * self.stride:y * self.stride + self.w] for y in range(self.h))
        # 32 bits a pixel, grey: one channel is enough
        return bytes(raw[y * self.stride + x * self.bytes + 1] for y in range(self.h) for x in range(self.w))

    def open(self):
        self.sh(f"cd {self.dir} && nohup sh launch.sh >/dev/null 2>&1 </dev/null &")
        return self.expect(r"shown in \d+ ms", 30)

    def close(self):
        self.tap(*self.menu)
        self.expect(r"menu drawn")
        self.tap(*self.leave)
        return self.expect(r"Nickel started again", 15)


def pgm(path, w, h, pix):
    with open(path, "wb") as f:
        f.write(b"P5 %d %d 255\n" % (w, h) + pix)


def read_pgm(path):
    data = open(path, "rb").read()
    return data[data.index(b"\n") + 1:]


def main():
    args = sys.argv[1:]
    save = "--save" in args
    args = [a for a in args if a != "--save"]
    if len(args) != 2 or args[0] not in DEVICES:
        sys.exit(__doc__)
    dev, path = Device(args[0]), args[1]
    name = os.path.splitext(os.path.basename(path))[0]
    out = os.path.join(os.path.dirname(os.path.abspath(__file__)), "out", name)
    refs = os.path.join(out, "refs")
    os.makedirs(refs, exist_ok=True)
    failed = 0
    for n, line in enumerate(open(path), 1):
        step = line.split("#")[0].split()
        if not step:
            continue
        op, a = step[0], step[1:]
        t = time.time()
        try:
            if op == "open":
                said = dev.open()
            elif op == "close":
                said = dev.close()
            elif op == "tap":
                dev.tap(int(a[0]), int(a[1]))
                said = ""
            elif op == "swipe":
                dev.swipe(*map(int, a))
                said = ""
            elif op == "hold":
                dev.hold(int(a[0]), int(a[1]), (int(a[3]), int(a[4])) if len(a) == 5 else None)
                said = ""
            elif op == "expect":
                said = dev.expect(" ".join(a))
            elif op == "wait":
                time.sleep(float(a[0]))
                said = ""
            elif op == "snap":
                pix = dev.screen()
                pgm(os.path.join(out, a[0] + ".pgm"), dev.w, dev.h, pix)
                ref = os.path.join(refs, a[0] + ".pgm")
                if save:
                    pgm(ref, dev.w, dev.h, pix)
                    said = "kept as the reference"
                elif not os.path.exists(ref):
                    said = "no reference (run with --save)"
                else:
                    old = read_pgm(ref)
                    skip = dev.top * dev.w
                    diff = sum(1 for p, q in zip(pix[skip:], old[skip:]) if p != q)
                    if diff:
                        raise AssertionError(f"{diff} pixels differ from the reference, below the clock's strip")
                    said = "identical to the reference"
            else:
                raise ValueError(f"unknown step {op!r}")
            print(f"ok   {n:3} {line.strip():50.50} {time.time() - t:5.1f} s  {said.strip()[:70]}")
        except Exception as e:
            failed += 1
            print(f"FAIL {n:3} {line.strip()}\n         {e}")
            break
    if failed:
        print("stopped at the first failure: the book may still be open on the device (close it, or: ssh "
              f"{dev.ssh} 'sh {dev.dir}/launch.sh nickel' once the reader has exited)")
    sys.exit(1 if failed else 0)


if __name__ == "__main__":
    main()
