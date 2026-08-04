import assert from 'node:assert/strict';
import fs from 'node:fs';
import test from 'node:test';
import ts from 'typescript';

const source = fs.readFileSync(new URL('./ansi.ts', import.meta.url), 'utf8');
const { outputText } = ts.transpileModule(source, {
  compilerOptions: { module: ts.ModuleKind.ESNext, target: ts.ScriptTarget.ES2022 },
});
const { ansiToHtml } = await import(`data:text/javascript;base64,${Buffer.from(outputText).toString('base64')}`);

test('plain text passes through unchanged', () => {
  assert.equal(ansiToHtml('hello\nworld'), 'hello\nworld');
});

test('escapes html in tool output', () => {
  assert.equal(ansiToHtml('<script>alert(1)</script>'), '&lt;script&gt;alert(1)&lt;/script&gt;');
});

test('renders standard colors as spans', () => {
  assert.equal(ansiToHtml('\x1b[31mred\x1b[0m plain'), '<span style="color:#ef4444">red</span> plain');
});

test('renders bright colors and bold', () => {
  assert.equal(
    ansiToHtml('\x1b[1;34m[~]\x1b[0m info'),
    '<span style="color:#3b82f6;font-weight:700">[~]</span> info',
  );
});

test('renders 24-bit truecolor from rustscan', () => {
  assert.equal(
    ansiToHtml('\x1b[38;2;0;255;0mgreen\x1b[0m'),
    '<span style="color:rgb(0,255,0)">green</span>',
  );
});

test('renders 256-color palette entries', () => {
  assert.equal(
    ansiToHtml('\x1b[38;5;196mred\x1b[39m'),
    '<span style="color:rgb(255,0,0)">red</span>',
  );
});

test('renders background colors', () => {
  assert.equal(
    ansiToHtml('\x1b[41malert\x1b[0m'),
    '<span style="background-color:#ef4444">alert</span>',
  );
});

test('keeps style open across lines until reset', () => {
  assert.equal(
    ansiToHtml('\x1b[32mline1\nline2\x1b[0m'),
    '<span style="color:#10b981">line1\nline2</span>',
  );
});

test('strips non-sgr control sequences', () => {
  assert.equal(ansiToHtml('\x1b[2K\x1b[1Gdone'), 'done');
});

test('strips osc sequences', () => {
  assert.equal(ansiToHtml('\x1b]0;window title\x07text'), 'text');
});

test('drops escape sequences at end of input safely', () => {
  assert.equal(ansiToHtml('partial \x1b[31'), 'partial ');
});

test('combines multiple sgr attributes', () => {
  assert.equal(
    ansiToHtml('\x1b[4;33mlink\x1b[0m'),
    '<span style="color:#d97706;text-decoration:underline">link</span>',
  );
});
