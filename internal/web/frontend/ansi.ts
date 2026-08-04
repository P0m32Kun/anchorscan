// ansiToHtml renders raw terminal output (ANSI SGR color sequences) as HTML
// for the single-tool page terminal, matching what an external terminal shows.
// All tool text is HTML-escaped before any span is emitted, so the result is
// safe to inject via v-html. Non-SGR escape sequences (cursor moves, OSC,
// erase) are stripped; text content is kept verbatim.

// Mid-tone palettes chosen to stay readable on both the light and dark
// code backgrounds used by the web console.
const STANDARD_COLORS = ['#6b7280', '#ef4444', '#10b981', '#d97706', '#3b82f6', '#a855f7', '#06b6d4', '#9ca3af'];
const BRIGHT_COLORS = ['#9ca3af', '#f87171', '#34d399', '#fbbf24', '#60a5fa', '#c084fc', '#22d3ee', '#e5e7eb'];

function color256(index: number): string {
  if (index < 8) return STANDARD_COLORS[index];
  if (index < 16) return BRIGHT_COLORS[index - 8];
  if (index < 232) {
    const value = index - 16;
    const channel = (level: number) => (level === 0 ? 0 : 55 + 40 * level);
    return `rgb(${channel(Math.floor(value / 36))},${channel(Math.floor((value % 36) / 6))},${channel(value % 6)})`;
  }
  const gray = 8 + 10 * (index - 232);
  return `rgb(${gray},${gray},${gray})`;
}

type Style = {
  fg?: string;
  bg?: string;
  bold?: boolean;
  dim?: boolean;
  italic?: boolean;
  underline?: boolean;
  strike?: boolean;
};

function applySgr(style: Style, params: number[]): void {
  const codes = params.length === 0 ? [0] : params;
  for (let i = 0; i < codes.length; i++) {
    const code = codes[i];
    if (code === 0) {
      for (const key of Object.keys(style) as (keyof Style)[]) delete style[key];
    } else if (code === 1) style.bold = true;
    else if (code === 2) style.dim = true;
    else if (code === 3) style.italic = true;
    else if (code === 4) style.underline = true;
    else if (code === 9) style.strike = true;
    else if (code === 22) { delete style.bold; delete style.dim; }
    else if (code === 23) delete style.italic;
    else if (code === 24) delete style.underline;
    else if (code === 29) delete style.strike;
    else if (code === 39) delete style.fg;
    else if (code === 49) delete style.bg;
    else if (code >= 30 && code <= 37) style.fg = STANDARD_COLORS[code - 30];
    else if (code >= 90 && code <= 97) style.fg = BRIGHT_COLORS[code - 90];
    else if (code >= 40 && code <= 47) style.bg = STANDARD_COLORS[code - 40];
    else if (code >= 100 && code <= 107) style.bg = BRIGHT_COLORS[code - 100];
    else if (code === 38 || code === 48) {
      const target: 'fg' | 'bg' = code === 38 ? 'fg' : 'bg';
      if (codes[i + 1] === 5 && codes[i + 2] !== undefined) {
        style[target] = color256(codes[i + 2]);
        i += 2;
      } else if (codes[i + 1] === 2 && codes[i + 2] !== undefined && codes[i + 3] !== undefined && codes[i + 4] !== undefined) {
        style[target] = `rgb(${codes[i + 2]},${codes[i + 3]},${codes[i + 4]})`;
        i += 4;
      }
    }
  }
}

function styleToCss(style: Style): string {
  const css: string[] = [];
  if (style.fg) css.push(`color:${style.fg}`);
  if (style.bg) css.push(`background-color:${style.bg}`);
  if (style.bold) css.push('font-weight:700');
  if (style.dim) css.push('opacity:0.7');
  if (style.italic) css.push('font-style:italic');
  const decorations = [style.underline ? 'underline' : '', style.strike ? 'line-through' : ''].filter(Boolean).join(' ');
  if (decorations) css.push(`text-decoration:${decorations}`);
  return css.join(';');
}

function escapeHtml(text: string): string {
  return text.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;');
}

export function ansiToHtml(input: string): string {
  let html = '';
  let text = '';
  let spanOpen = false;
  const style: Style = {};

  const flushText = () => {
    if (text) {
      html += escapeHtml(text);
      text = '';
    }
  };
  const restyle = () => {
    flushText();
    if (spanOpen) {
      html += '</span>';
      spanOpen = false;
    }
    const css = styleToCss(style);
    if (css) {
      html += `<span style="${css}">`;
      spanOpen = true;
    }
  };

  let i = 0;
  while (i < input.length) {
    const char = input[i];
    if (char !== '\x1b') {
      text += char;
      i += 1;
      continue;
    }
    const introducer = input[i + 1];
    if (introducer === ']') {
      // OSC: skip until BEL or ESC \
      let end = i + 2;
      while (end < input.length && input[end] !== '\x07' && !(input[end] === '\x1b' && input[end + 1] === '\\')) end += 1;
      i = input[end] === '\x07' ? end + 1 : end + 2;
      continue;
    }
    if (introducer === '[') {
      let end = i + 2;
      while (end < input.length && !(input[end] >= '@' && input[end] <= '~')) end += 1;
      const finalByte = input[end];
      if (finalByte === undefined) break;
      if (finalByte === 'm') {
        const params = input
          .slice(i + 2, end)
          .split(';')
          .filter((part) => part !== '')
          .map((part) => Number.parseInt(part, 10))
          .filter((part) => Number.isFinite(part));
        applySgr(style, params);
        restyle();
      }
      // Other CSI sequences (cursor moves, erase) are dropped.
      i = end + 1;
      continue;
    }
    // Remaining escape forms: ESC plus one optional intermediate plus final byte.
    i += introducer && '()#*'.includes(introducer) ? 3 : 2;
  }
  flushText();
  if (spanOpen) html += '</span>';
  return html;
}
