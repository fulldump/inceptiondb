// Conversion between the visual filter builder (a list of conditions) and the
// JSON filter sent to the API. The JSON text stays the source of truth.

import { parseLooseValue } from './format.js';

// Operators understood by the server (equality is evaluated natively, the
// rest by its connor matcher).
export const OPERATORS = [
  { value: 'eq', label: '=' },
  { value: 'ne', label: '≠' },
  { value: 'gt', label: '>' },
  { value: 'ge', label: '≥' },
  { value: 'lt', label: '<' },
  { value: 'le', label: '≤' },
  { value: 'in', label: 'in' },
  { value: 'nin', label: 'not in' },
  { value: 'contains', label: 'contains' },
];

const OPERATOR_SET = new Set(OPERATORS.map((o) => o.value));
const LIST_OPERATORS = new Set(['in', 'nin']);

const isPlainObject = (v) => v !== null && typeof v === 'object' && !Array.isArray(v);

const formatValue = (value) => (typeof value === 'string' ? value : JSON.stringify(value));

// conditionsFromFilter returns the conditions of a filter, or null when the
// filter cannot be represented with the builder.
export const conditionsFromFilter = (filter) => {
  if (!isPlainObject(filter)) return null;
  const conditions = [];
  for (const [field, expected] of Object.entries(filter)) {
    if (field.startsWith('$')) return null;
    if (!isPlainObject(expected)) {
      conditions.push({ field, op: 'eq', value: formatValue(expected) });
      continue;
    }
    const entries = Object.entries(expected);
    if (entries.length === 0) return null;
    for (const [key, value] of entries) {
      const op = key.replace(/^\$/, '');
      if (!key.startsWith('$') || !OPERATOR_SET.has(op)) return null;
      if (LIST_OPERATORS.has(op)) {
        if (!Array.isArray(value)) return null;
        conditions.push({ field, op, value: value.map(formatValue).join(', ') });
      } else {
        if (isPlainObject(value) || Array.isArray(value)) return null;
        conditions.push({ field, op, value: formatValue(value) });
      }
    }
  }
  return conditions;
};

const conditionValue = ({ op, value }) => {
  if (op === 'contains') return String(value);
  if (LIST_OPERATORS.has(op)) {
    return String(value)
      .split(',')
      .map((part) => part.trim())
      .filter((part) => part !== '')
      .map(parseLooseValue);
  }
  return parseLooseValue(value);
};

// filterFromConditions builds the JSON filter. Conditions without a field are
// ignored; several conditions on a field are combined.
export const filterFromConditions = (conditions) => {
  const filter = {};
  for (const condition of conditions) {
    const field = condition.field.trim();
    if (!field) continue;
    const value = conditionValue(condition);
    if (condition.op === 'eq' && !(field in filter)) {
      filter[field] = value;
      continue;
    }
    // Move an existing equality into operator form to combine it
    if (field in filter && !isPlainObject(filter[field])) filter[field] = { $eq: filter[field] };
    filter[field] = { ...(filter[field] || {}), [`$${condition.op}`]: value };
  }
  return filter;
};

export const filterText = (filter) => (Object.keys(filter).length ? JSON.stringify(filter, null, 2) : '{\n\n}');
