// CSV import helpers: delimiter detection, parsing and conversion to documents.

export const CSV_DELIMITERS = [
  { value: ',', label: 'Comma (,)' },
  { value: ';', label: 'Semicolon (;)' },
  { value: '\t', label: 'Tab' },
  { value: '|', label: 'Pipe (|)' },
];

export const detectCsvDelimiter = (sample) => {
  if (typeof sample !== 'string' || !sample) return null;
  const lines = sample.split(/\r?\n/).slice(0, 5);
  let best = null;
  let bestScore = 0;
  CSV_DELIMITERS.forEach(({ value }) => {
    let score = 0;
    lines.forEach((line) => {
      if (line) score += line.split(value).length - 1;
    });
    if (score > bestScore) {
      bestScore = score;
      best = value;
    }
  });
  return best;
};

// parseCsvRows supports quoted values, escaped quotes ("") and CRLF.
export const parseCsvRows = (text, delimiter) => {
  const rows = [];
  const sep = typeof delimiter === 'string' && delimiter.length > 0 ? delimiter : ',';
  let value = '';
  let row = [];
  let inQuotes = false;
  for (let i = 0; i < text.length; i += 1) {
    const char = text[i];
    if (inQuotes) {
      if (char === '"') {
        if (text[i + 1] === '"') {
          value += '"';
          i += 1;
        } else {
          inQuotes = false;
        }
      } else {
        value += char;
      }
    } else if (char === '"') {
      inQuotes = true;
    } else if (char === sep) {
      row.push(value);
      value = '';
    } else if (char === '\n') {
      row.push(value);
      rows.push(row);
      row = [];
      value = '';
    } else if (char !== '\r') {
      value += char;
    }
  }
  if (inQuotes) throw new Error('Unclosed quoted value in CSV data.');
  if (value !== '' || row.length > 0) {
    row.push(value);
    rows.push(row);
  }
  return rows;
};

const isBlankRow = (row) => row.every((cell) => cell === null || cell === undefined || String(cell).trim() === '');

// csvToDocuments turns CSV text into documents, using the first row as field names.
export const csvToDocuments = (text, delimiter, trimValues) => {
  const rows = parseCsvRows(text.replace(/^﻿/, ''), delimiter).filter(
    (row, index) => Array.isArray(row) && (index === 0 || !isBlankRow(row)),
  );
  if (rows.length <= 1) {
    throw new Error('The CSV file must include a header row and at least one data row.');
  }
  const headers = rows[0].map((cell, idx) => {
    const header = typeof cell === 'string' ? cell.trim() : '';
    if (!header) throw new Error(`Column ${idx + 1} header is empty.`);
    return header;
  });
  return rows.slice(1).map((row) => {
    const doc = {};
    headers.forEach((header, idx) => {
      let cell = row[idx] ?? '';
      if (trimValues && typeof cell === 'string') cell = cell.trim();
      doc[header] = cell;
    });
    return doc;
  });
};
