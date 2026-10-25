// Code snippets that reproduce a logged request in several languages.
// A request is { method, url, headers: {name: value}, body } where body is a
// string or a JSON value.

const absoluteUrl = (url) => {
  try {
    return new URL(url, window.location.origin).toString();
  } catch (err) {
    return url;
  }
};

const headerList = (request) =>
  Object.entries(request.headers || {})
    .filter(([key, value]) => key && value !== undefined && value !== null)
    .map(([key, value]) => ({ key, value: String(value) }));

const hasBody = (request) => request.body !== undefined && request.body !== null && request.body !== '';

const bodyText = (request, pretty) => {
  if (typeof request.body === 'string') return request.body;
  try {
    return pretty ? JSON.stringify(request.body, null, 2) : JSON.stringify(request.body);
  } catch (err) {
    return String(request.body);
  }
};

// Removes the trailing comma of the last line in lines.
const dropTrailingComma = (lines) => {
  const last = lines.length - 1;
  lines[last] = lines[last].replace(/,$/, '');
};

const http = (request) => {
  let path = request.url;
  let host = '';
  try {
    const parsed = new URL(request.url, window.location.origin);
    host = parsed.host;
    path = `${parsed.pathname || '/'}${parsed.search || ''}`;
  } catch (err) {
    // keep the raw url
  }
  const lines = [`${request.method} ${path || '/'} HTTP/1.1`];
  if (host) lines.push(`Host: ${host}`);
  headerList(request).forEach(({ key, value }) => lines.push(`${key}: ${value}`));
  if (hasBody(request)) lines.push('', bodyText(request, true));
  return lines.join('\n');
};

const javascript = (request) => {
  const lines = [`const response = await fetch(${JSON.stringify(absoluteUrl(request.url))}, {`, `  method: '${request.method}',`];
  const headers = headerList(request);
  if (headers.length > 0) {
    lines.push('  headers: {');
    headers.forEach(({ key, value }) => lines.push(`    ${JSON.stringify(key)}: ${JSON.stringify(value)},`));
    dropTrailingComma(lines);
    lines.push('  },');
  }
  if (hasBody(request)) {
    if (typeof request.body === 'string') {
      lines.push(`  body: ${JSON.stringify(request.body)},`);
    } else {
      lines.push('  body: JSON.stringify(');
      bodyText(request, true).split('\n').forEach((line) => lines.push(`    ${line}`));
      lines.push('  ),');
    }
  }
  dropTrailingComma(lines);
  lines.push('});', '', 'if (!response.ok) {', "  throw new Error('Request failed');", '}', '', 'const data = await response.text();', 'console.log(data);');
  return lines.join('\n');
};

const golang = (request) => {
  const imports = new Set(['"fmt"', '"io"', '"log"', '"net/http"']);
  let bodyRef = 'nil';
  let bodyDecl = '';
  if (hasBody(request)) {
    imports.add('"strings"');
    bodyDecl = `\tpayload := strings.NewReader(${JSON.stringify(bodyText(request, true))})`;
    bodyRef = 'payload';
  }
  const lines = ['package main', '', 'import ('];
  Array.from(imports).sort().forEach((imp) => lines.push(`\t${imp}`));
  lines.push(')', '', 'func main() {', '\tclient := &http.Client{}');
  if (bodyDecl) lines.push(bodyDecl);
  lines.push(`\treq, err := http.NewRequest(${JSON.stringify(request.method)}, ${JSON.stringify(absoluteUrl(request.url))}, ${bodyRef})`);
  lines.push('\tif err != nil {', '\t\tlog.Fatal(err)', '\t}');
  const headers = headerList(request);
  if (headers.length > 0) {
    lines.push('');
    headers.forEach(({ key, value }) => lines.push(`\treq.Header.Set(${JSON.stringify(key)}, ${JSON.stringify(value)})`));
  }
  lines.push(
    '', '\tresp, err := client.Do(req)', '\tif err != nil {', '\t\tlog.Fatal(err)', '\t}', '\tdefer resp.Body.Close()',
    '', '\tbody, err := io.ReadAll(resp.Body)', '\tif err != nil {', '\t\tlog.Fatal(err)', '\t}', '\tfmt.Println(string(body))', '}',
  );
  return lines.join('\n');
};

const python = (request) => {
  const lines = ['import requests'];
  let payloadArg = '';
  if (hasBody(request)) {
    if (typeof request.body === 'string') {
      lines.push('', `payload = ${JSON.stringify(request.body)}`);
      payloadArg = 'data=payload';
    } else {
      lines.unshift('import json');
      lines.push('', 'payload = json.loads("""', ...bodyText(request, true).split('\n'), '""")');
      payloadArg = 'json=payload';
    }
  }
  lines.push('', `url = ${JSON.stringify(absoluteUrl(request.url))}`);
  const headers = headerList(request);
  if (headers.length > 0) {
    lines.push('headers = {');
    headers.forEach(({ key, value }) => lines.push(`    ${JSON.stringify(key)}: ${JSON.stringify(value)},`));
    dropTrailingComma(lines);
    lines.push('}');
  }
  const args = [JSON.stringify(request.method), 'url'];
  if (headers.length > 0) args.push('headers=headers');
  if (payloadArg) args.push(payloadArg);
  lines.push('', `response = requests.request(${args.join(', ')})`, 'print(response.text)');
  return lines.join('\n');
};

const php = (request) => {
  const lines = ['<?php', '', '$curl = curl_init();', '', 'curl_setopt_array($curl, ['];
  lines.push(`    CURLOPT_URL => ${JSON.stringify(absoluteUrl(request.url))},`);
  lines.push('    CURLOPT_RETURNTRANSFER => true,');
  lines.push(`    CURLOPT_CUSTOMREQUEST => ${JSON.stringify(request.method)},`);
  const headers = headerList(request);
  if (headers.length > 0) {
    lines.push('    CURLOPT_HTTPHEADER => [');
    headers.forEach(({ key, value }) => lines.push(`        ${JSON.stringify(`${key}: ${value}`)},`));
    dropTrailingComma(lines);
    lines.push('    ],');
  }
  if (hasBody(request)) lines.push(`    CURLOPT_POSTFIELDS => ${JSON.stringify(bodyText(request, false))},`);
  lines.push(']);', '', '$response = curl_exec($curl);', '', 'if ($response === false) {', '    throw new RuntimeException(curl_error($curl));', '}', '', 'curl_close($curl);', '', 'echo $response;');
  return lines.join('\n');
};

const shellQuote = (value) => `'${String(value).replace(/'/g, `'"'"'`)}'`;

const curl = (request) => {
  const lines = [`curl -X ${request.method} ${shellQuote(absoluteUrl(request.url))}`];
  headerList(request).forEach(({ key, value }) => lines.push(`  -H ${shellQuote(`${key}: ${value}`)}`));
  if (hasBody(request)) lines.push(`  --data-raw ${shellQuote(bodyText(request, false))}`);
  return lines.join(' \\\n');
};

export const SNIPPETS = [
  { id: 'http', label: 'HTTP', build: http },
  { id: 'javascript', label: 'JavaScript', build: javascript },
  { id: 'golang', label: 'Go', build: golang },
  { id: 'python', label: 'Python', build: python },
  { id: 'php', label: 'PHP', build: php },
  { id: 'curl', label: 'cURL', build: curl },
];
