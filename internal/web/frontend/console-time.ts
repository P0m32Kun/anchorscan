const shanghaiFormatter = new Intl.DateTimeFormat('en-CA', {
  timeZone: 'Asia/Shanghai',
  year: 'numeric',
  month: '2-digit',
  day: '2-digit',
  hour: '2-digit',
  minute: '2-digit',
  second: '2-digit',
  fractionalSecondDigits: 3,
  hourCycle: 'h23',
});

export function formatConsoleTime(value: string): string {
  const time = new Date(value);
  if (Number.isNaN(time.getTime())) return `invalid time: ${value}`;
  const parts = Object.fromEntries(shanghaiFormatter.formatToParts(time).map(({ type, value: part }) => [type, part]));
  return `${parts.year}-${parts.month}-${parts.day} ${parts.hour}:${parts.minute}:${parts.second}.${parts.fractionalSecond} UTC+8`;
}
