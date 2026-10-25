// Formatting helpers shared by the components.

const numberFormatter = new Intl.NumberFormat('en-US');
const timeFormatter = new Intl.DateTimeFormat('en-US', {
  hour: 'numeric',
  minute: '2-digit',
  second: '2-digit',
  hour12: true,
});

export const formatNumber = (n) => numberFormatter.format(n);

export const formatTime = (date) => (date instanceof Date ? timeFormatter.format(date) : '');

export const prettyTotal = (n) => {
  if (typeof n !== 'number') return n;
  if (n >= 1_000_000) return (n / 1_000_000).toFixed(1).replace(/\.0$/, '') + 'M';
  if (n >= 1_000) return (n / 1_000).toFixed(1).replace(/\.0$/, '') + 'K';
  return String(n);
};

export const formatDuration = (ms) => {
  if (typeof ms !== 'number' || Number.isNaN(ms)) return '';
  if (ms < 1000) return `${ms} ms`;
  if (ms < 10000) return `${(ms / 1000).toFixed(2)} s`;
  return `${(ms / 1000).toFixed(1)} s`;
};

export const formatBytes = (value) => {
  const bytes = Number(value);
  if (!Number.isFinite(bytes)) return value === null || value === undefined ? '' : String(value);
  const abs = Math.abs(bytes);
  if (abs < 1024) return `${numberFormatter.format(bytes)} B`;
  const units = ['B', 'KB', 'MB', 'GB', 'TB', 'PB'];
  const exponent = Math.min(Math.floor(Math.log(abs) / Math.log(1024)), units.length - 1);
  const scaled = bytes / 1024 ** exponent;
  const formatted = Math.abs(scaled) >= 10 ? scaled.toFixed(0) : scaled.toFixed(1);
  return `${formatted.replace(/\.0$/, '')} ${units[exponent]}`;
};

export const toNonNegativeInteger = (value, fallback = 0) => {
  const num = Number(value);
  if (!Number.isFinite(num)) return fallback;
  return Math.max(0, Math.floor(num));
};

export const pluralize = (count, singular, plural = `${singular}s`) =>
  `${formatNumber(count)} ${count === 1 ? singular : plural}`;

export const prettyJson = (value) => {
  try {
    return JSON.stringify(value, null, 2);
  } catch (err) {
    return String(value);
  }
};

const escapeHtml = (text) =>
  text.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;');

// highlightJson returns pretty printed JSON as HTML with token classes. Input
// is escaped before any markup is added.
export const highlightJson = (value) =>
  escapeHtml(prettyJson(value)).replace(
    /("(?:\\.|[^"\\])*")(\s*:)?|\b(true|false|null)\b|(-?\d+(?:\.\d+)?(?:[eE][+-]?\d+)?)/g,
    (match, str, colon, literal, number) => {
      if (str) return colon ? `<span class="json-key">${str}</span>${colon}` : `<span class="json-string">${str}</span>`;
      if (literal) return `<span class="json-literal">${literal}</span>`;
      return `<span class="json-number">${number}</span>`;
    },
  );

export const truncate = (text, max = 160) => (text.length > max ? `${text.slice(0, max - 3)}…` : text);

// documentId returns the "id" of a document as a string, or null.
export const documentId = (row) => {
  if (!row || typeof row !== 'object' || Array.isArray(row)) return null;
  if (Object.prototype.hasOwnProperty.call(row, '_raw')) return null;
  if (!Object.prototype.hasOwnProperty.call(row, 'id')) return null;
  const value = row.id;
  if (value === undefined || value === null) return null;
  return String(value);
};

// parseLooseValue reads what a user typed in a value field: valid JSON
// (numbers, booleans, quoted strings...) keeps its type, anything else is a
// string. "42" is the number 42, "Ada" is the string "Ada".
export const parseLooseValue = (text) => {
  const trimmed = String(text).trim();
  if (trimmed === '') return '';
  try {
    return JSON.parse(trimmed);
  } catch (err) {
    return trimmed;
  }
};
