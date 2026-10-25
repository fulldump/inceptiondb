// Shared state: API client, connection status, activity log, collections and
// UI preferences.

const { reactive } = Vue;

// ---------- connection status ----------

export const connection = reactive({
  state: 'unknown', // unknown | online | degraded | offline
  detail: 'Waiting for first response.',
  checkedAt: null,
  checking: false,
});

const setConnection = (state, detail) => {
  connection.state = state;
  connection.detail = detail;
  connection.checkedAt = new Date();
  connection.checking = false;
};

// ---------- activity log ----------

const MAX_ACTIVITY_ENTRIES = 15;

export const activity = reactive({ entries: [] });

let activitySeq = 0;

const logRequest = (label, method, url, body, headers, background) => {
  const entry = reactive({
    id: ++activitySeq,
    label,
    background, // automatic refresh, not a user action
    status: 'pending', // pending | success | error
    statusCode: null,
    startedAt: Date.now(),
    durationMs: null,
    request: { method, url, headers, body: body === undefined ? null : body },
  });
  activity.entries.unshift(entry);
  activity.entries.splice(MAX_ACTIVITY_ENTRIES);
  return entry;
};

const finishRequest = (entry, status, statusCode = null) => {
  if (!entry) return;
  entry.status = status;
  entry.statusCode = statusCode;
  entry.durationMs = Date.now() - entry.startedAt;
};

// ---------- API client ----------

export class ApiError extends Error {
  constructor(message, status = 0, data = null) {
    super(message);
    this.status = status;
    this.data = data;
  }
}

const errorMessage = (data, status) => {
  const error = data && typeof data === 'object' ? data.error : null;
  if (error && typeof error === 'object') return error.message || error.description || `HTTP ${status}`;
  if (typeof error === 'string') return error;
  if (typeof data === 'string' && data.trim()) return data.trim();
  return `HTTP ${status}`;
};

const prepare = (body) => {
  if (body === undefined || body === null) return { payload: undefined, headers: {} };
  // NDJSON strings are sent as they are, like any JSON body.
  const payload = typeof body === 'string' ? body : JSON.stringify(body);
  return { payload, headers: { 'Content-Type': 'application/json' } };
};

/**
 * api performs a request and records it in the activity log.
 * Options: body (JSON value or string), text (return the raw body),
 * log (record in the activity log, default true), stream (return the Response),
 * background (an automatic refresh rather than a user action).
 */
export async function api(label, method, url, { body, text = false, log = true, stream = false, background = false } = {}) {
  const { payload, headers } = prepare(body);
  const entry = log ? logRequest(label, method, url, body, headers, background) : null;

  let response;
  try {
    response = await fetch(url, { method, headers, body: payload });
  } catch (err) {
    finishRequest(entry, 'error');
    setConnection('offline', `Network error: ${err.message}`);
    throw new ApiError(`Unable to reach the API: ${err.message}`);
  }

  if (stream && response.ok) {
    finishRequest(entry, 'success', response.status);
    setConnection('online', 'All systems operational');
    return response;
  }

  const raw = await response.text();
  let data = raw;
  if (!text) {
    try {
      data = raw ? JSON.parse(raw) : null;
    } catch (err) {
      data = raw;
    }
  }

  if (!response.ok) {
    finishRequest(entry, 'error', response.status);
    if (response.status >= 500) setConnection('degraded', `Server error (HTTP ${response.status})`);
    let parsed = data;
    if (text) {
      try {
        parsed = JSON.parse(raw);
      } catch (err) {
        // keep raw text
      }
    }
    throw new ApiError(errorMessage(parsed, response.status), response.status, parsed);
  }

  finishRequest(entry, 'success', response.status);
  setConnection('online', 'All systems operational');
  return { status: response.status, data };
}

export const collectionUrl = (name, action = '') =>
  `/v1/collections/${encodeURIComponent(name)}${action ? `:${action}` : ''}`;

// parseNdjson parses one document per line; invalid lines are kept as {_raw}.
export const parseNdjson = (text) =>
  String(text || '')
    .split('\n')
    .filter(Boolean)
    .map((line) => {
      let doc;
      try {
        doc = JSON.parse(line);
      } catch (err) {
        doc = { _raw: line };
      }
      if (doc && typeof doc === 'object') {
        Object.defineProperty(doc, '__rawLine', { value: line, enumerable: false });
      }
      return doc;
    });

let statusPromise = null;

export const checkConnection = () => {
  if (statusPromise) return statusPromise;
  connection.checking = true;
  if (connection.state === 'unknown') connection.detail = 'Checking backend status…';
  statusPromise = api('Health check', 'GET', '/v1/collections', { log: false })
    .catch((err) => {
      if (err.status && err.status < 500) setConnection('degraded', `Unexpected response (HTTP ${err.status})`);
    })
    .finally(() => {
      connection.checking = false;
      statusPromise = null;
    });
  return statusPromise;
};

// ---------- collections ----------

export const collections = reactive({ list: [], loading: false, error: '' });

export const loadCollections = async () => {
  collections.loading = true;
  collections.error = '';
  try {
    const { data } = await api('List collections', 'GET', '/v1/collections', { background: true });
    const list = Array.isArray(data) ? data.slice() : [];
    list.sort((a, b) => a.name.localeCompare(b.name));
    collections.list = list;
  } catch (err) {
    collections.error = err.message || 'Failed to load collections.';
  } finally {
    collections.loading = false;
  }
};

export const findCollection = (name) => collections.list.find((c) => c.name === name) || null;

// ---------- UI preferences (kept while switching collections) ----------

export const ui = reactive({
  resultsView: 'table', // table | json
  activityOpen: false,
  activityEntry: null, // request shown when the log opens, null for the list
});

/** showRequest opens the request log on one request (or on the list). */
export const showRequest = (entry = null) => {
  ui.activityEntry = entry;
  ui.activityOpen = true;
};

// ---------- notifications ----------

export const toasts = reactive({ list: [] });

let toastSeq = 0;

// Requests that finished this recently are the ones a notification is about.
const TOAST_REQUEST_WINDOW_MS = 3000;

/**
 * notify shows a transient message. kind: success | error | info.
 * Success and error messages link to the request that caused them, so the
 * user can see how to make the same call from code.
 */
export const notify = (kind, message, timeout = kind === 'error' ? 8000 : 5000) => {
  const id = ++toastSeq;
  const last = activity.entries.find((e) => !e.background && e.status !== 'pending');
  const finishedAt = last ? last.startedAt + (last.durationMs || 0) : 0;
  const entry = kind !== 'info' && last && Date.now() - finishedAt < TOAST_REQUEST_WINDOW_MS ? last : null;
  toasts.list.push({ id, kind, message, entry });
  toasts.list.splice(0, Math.max(0, toasts.list.length - 4));
  setTimeout(() => dismiss(id), timeout);
};

export const dismiss = (id) => {
  const i = toasts.list.findIndex((t) => t.id === id);
  if (i >= 0) toasts.list.splice(i, 1);
};

// ---------- confirmation dialog ----------

export const confirmation = reactive({ open: false, title: '', message: '', choices: [], resolve: null });

/**
 * confirmAction opens the confirmation dialog and resolves with the value of
 * the chosen option, or null when cancelled.
 * choices: [{ value, label, kind: 'primary' | 'danger' | '' }]
 */
export const confirmAction = ({ title, message = '', choices }) =>
  new Promise((resolve) => {
    if (confirmation.resolve) confirmation.resolve(null);
    Object.assign(confirmation, { open: true, title, message, choices, resolve });
  });

export const confirmDanger = async (title, message, label = 'Delete') =>
  (await confirmAction({ title, message, choices: [{ value: true, label, kind: 'danger' }] })) === true;
