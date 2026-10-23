// Where each word span went, once the fonts are in: [id, top, left, right] in CSS px, written into the page so
// chromium --dump-dom prints it.
document.fonts.ready.then(() => {
  const out = [];
  for (const s of document.querySelectorAll('span[id^="w"]')) {
    const r = s.getClientRects()[0];
    if (r) out.push([s.id, Math.round(r.top + scrollY), Math.round(r.left), Math.round(r.right)]);
  }
  const pre = document.createElement("pre");
  pre.id = "measure-out";
  pre.textContent = JSON.stringify(out);
  document.body.appendChild(pre);
});
