// The view controls on both pages: the device's colour (white or black) and the screen by day or at night.
// Kept in localStorage when it is there, so the choice follows from one page to the other.
(function () {
  const root = document.documentElement;
  const read = k => { try { return localStorage.getItem(k); } catch (e) { return null; } };
  const write = (k, v) => { try { localStorage.setItem(k, v); } catch (e) {} };
  const picks = [
    { key: 'device', label: 'Device', def: 'white', opts: [['white', 'White'], ['black', 'Black']] },
    { key: 'screen', label: 'Screen', def: 'day', opts: [['day', 'Day'], ['night', 'Night']] },
  ];
  const bar = document.createElement('div');
  bar.className = 'view';
  picks.forEach(p => {
    const wrap = document.createElement('div');
    wrap.className = 'pick';
    wrap.innerHTML = '<span>' + p.label + '</span><div class="seg" role="group" aria-label="' + p.label + '"></div>';
    const seg = wrap.lastChild;
    const apply = v => {
      root.setAttribute('data-' + p.key, v);
      seg.querySelectorAll('button').forEach(b => b.setAttribute('aria-pressed', b.dataset.v === v));
    };
    p.opts.forEach(([v, text]) => {
      const b = document.createElement('button');
      b.type = 'button';
      b.dataset.v = v;
      b.textContent = text;
      b.addEventListener('click', () => { apply(v); write(p.key, v); });
      seg.appendChild(b);
    });
    const saved = read(p.key);
    apply(p.opts.some(o => o[0] === saved) ? saved : p.def);
    bar.appendChild(wrap);
  });
  // beside the outlines checkbox where a page has that bar, else under the tabs
  const home = document.querySelector('.bar label.sw');
  if (home) { bar.classList.add('inbar'); home.insertAdjacentElement('afterend', bar); }
  else document.querySelector('nav.tabs-top').insertAdjacentElement('afterend', bar);
})();
