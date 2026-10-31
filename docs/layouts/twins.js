// layout.html: beside each screen, its right to left twin (Farsi), to see both at once. The twin is a copy of the
// screen: every layer dir=rtl, the interface's Farsi words where the reader has them (words.js, made from
// reader/ui/words.go by gen-words.py), Farsi digits; the book's page gets the Farsi sample text of the page that
// is already Farsi. Book titles and words the reader has no Farsi for stay as drawn: a gap to fill. A screen that is
// Farsi already (its own figure) gets no twin.
(function () {
  const esc = t => t.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
  const exact = new Map(), pats = [];
  [...(window.WORDS || [])].sort((a, b) => b[0].length - a[0].length).forEach(([en, fa]) => { // most specific first
    if (!/%[ds]/.test(en)) { if (!exact.has(en)) exact.set(en, fa); return; }
    pats.push([new RegExp('^' + esc(en).replace(/%[ds]/g, '(.+?)') + '$'), fa]);
  });
  const digits = t => t.replace(/\d/g, d => '۰۱۲۳۴۵۶۷۸۹'[d]).replace(/%/g, '٪');
  const farsi = t => {
    const m = t.match(/^(\s*)(.*?)(\s*)$/s);
    if (!m[2]) return t;
    let out = null;
    if (exact.has(m[2])) out = exact.get(m[2]);
    else for (const [re, fa] of pats) {
      const x = m[2].match(re);
      if (x) { let i = 0; out = fa.replace(/%[ds]/g, () => x[++i]); break; }
    }
    return m[1] + digits(out === null ? m[2] : out) + m[3];
  };
  const sample = document.querySelector('.page[dir="rtl"]');
  document.querySelectorAll('figure').forEach(fig => {
    const dev = fig.querySelector(':scope > .device');
    if (!dev || dev.querySelector('.layer[dir="rtl"], .page[dir="rtl"]')) return;
    if ([...dev.querySelectorAll('.layer')].some(l => (l.dataset.layer || '').startsWith('library'))) return; // always English
    const twin = dev.cloneNode(true);
    twin.classList.add('twin');
    twin.setAttribute('aria-hidden', 'true');
    twin.querySelectorAll('[id]').forEach(e => e.removeAttribute('id'));
    twin.querySelector('.screen').setAttribute('dir', 'rtl'); // so the stacks and the menus' flex ends flip too
    twin.querySelectorAll('.layer').forEach(l => { l.setAttribute('dir', 'rtl'); l.setAttribute('lang', 'fa'); });
    twin.querySelectorAll('.layer, .layer *').forEach(el => el.childNodes.forEach(n => {
      if (n.nodeType === 3 && n.nodeValue.trim()) n.nodeValue = farsi(n.nodeValue);
    }));
    twin.querySelectorAll('.page').forEach(p => {
      if (!sample) return;
      p.setAttribute('dir', 'rtl'); p.setAttribute('lang', 'fa');
      p.innerHTML = sample.innerHTML;
    });
    const pair = document.createElement('div');
    pair.className = 'pair';
    const side = (el, name) => {
      const s = document.createElement('div');
      s.className = 'side';
      s.appendChild(el);
      const t = document.createElement('small');
      t.textContent = name;
      s.appendChild(t);
      return s;
    };
    dev.replaceWith(pair);
    pair.append(side(dev, 'English · LTR'), side(twin, 'Farsi · RTL'));
    fig.classList.add('paired');
  });
})();
