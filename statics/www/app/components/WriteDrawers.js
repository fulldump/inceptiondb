// Side panels that write documents: insert JSON and import CSV or JSONL files.

import { api, ApiError, collectionUrl, notify, parseNdjson } from '../store.js';
import { CSV_DELIMITERS, csvToDocuments, detectCsvDelimiter } from '../csv.js';
import { formatNumber, pluralize, truncate } from '../format.js';
import { COLLECTION_KEY } from '../query.js';
import { Icon } from '../icons.js';
import { AppDialog } from './common.js';

const { computed, inject, reactive, ref, watch } = Vue;

// parseDocuments accepts a JSON object, a JSON array or JSON Lines.
export const parseDocuments = (text) => {
  const trimmed = text.trim();
  if (!trimmed) return { docs: [], error: '' };
  try {
    const value = JSON.parse(trimmed);
    const docs = Array.isArray(value) ? value : [value];
    const bad = docs.findIndex((d) => !d || typeof d !== 'object' || Array.isArray(d));
    if (bad >= 0) return { docs: [], error: `Item ${bad + 1} is not a JSON object.` };
    return { docs, error: '' };
  } catch (err) {
    // Not a single JSON value: try one document per line.
  }
  const docs = [];
  const lines = trimmed.split('\n');
  for (let i = 0; i < lines.length; i += 1) {
    const line = lines[i].trim();
    if (!line) continue;
    try {
      const doc = JSON.parse(line);
      if (!doc || typeof doc !== 'object' || Array.isArray(doc)) return { docs: [], error: `Line ${i + 1} is not a JSON object.` };
      docs.push(doc);
    } catch (err) {
      return { docs: [], error: `Line ${i + 1}: ${err.message}` };
    }
  }
  return { docs, error: '' };
};

// insertDocuments sends documents as JSON Lines. The server answers with the
// stored documents and stops at the first rejected one; in that case the last
// line is {"error": {...}} with the reason.
const insertDocuments = async (collection, docs, label) => {
  const body = docs.map((d) => JSON.stringify(d)).join('\n');
  const { data } = await api(label, 'POST', collectionUrl(collection, 'insert'), { body, text: true });
  const lines = parseNdjson(data);
  const last = lines[lines.length - 1];
  const failed = last && Object.keys(last).length === 1 && last.error && typeof last.error === 'object';
  return {
    stored: failed ? lines.length - 1 : lines.length,
    reason: failed ? last.error.message || last.error.description || '' : '',
  };
};

const reportInsert = ({ stored, reason }, total) => {
  if (stored === total) {
    notify('success', `Inserted ${pluralize(total, 'document')}.`);
    return;
  }
  const why = reason ? `: ${reason}` : '';
  notify('error', `Inserted ${formatNumber(stored)} of ${formatNumber(total)} documents. Document ${stored + 1} was rejected${why}. The documents after it were not inserted.`);
};

// Imports are sent in batches so big files show progress and do not build a
// single huge request.
const IMPORT_BATCH_SIZE = 5000;

/**
 * insertInBatches inserts docs in order and stops at the first rejected
 * document, like a single request would. onProgress(stored) runs after each batch.
 */
const insertInBatches = async (collection, docs, label, onProgress) => {
  let stored = 0;
  for (let start = 0; start < docs.length; start += IMPORT_BATCH_SIZE) {
    const batch = docs.slice(start, start + IMPORT_BATCH_SIZE);
    let result;
    try {
      result = await insertDocuments(collection, batch, label);
    } catch (err) {
      // The first document of the batch was rejected (HTTP 4xx).
      if (err instanceof ApiError && err.status >= 400 && err.status < 500) return { stored, reason: err.message };
      throw err;
    }
    stored += result.stored;
    onProgress(stored);
    if (result.stored < batch.length) return { stored, reason: result.reason };
  }
  return { stored, reason: '' };
};

export const InsertDrawer = {
  name: 'InsertDrawer',
  components: { AppDialog, Icon },
  props: { open: { type: Boolean, default: false } },
  emits: ['update:open'],
  setup(props, { emit }) {
    const ctx = inject(COLLECTION_KEY);
    const form = reactive({ text: '', saving: false, error: '' });
    const parsed = computed(() => parseDocuments(form.text));
    const defaults = computed(() => ctx.collection.value?.defaults || {});

    watch(
      () => props.open,
      (open) => {
        if (open) Object.assign(form, { text: '{\n  \n}', saving: false, error: '' });
      },
    );

    const submit = async () => {
      form.error = parsed.value.error;
      if (form.error) return;
      if (!parsed.value.docs.length) {
        form.error = 'Write at least one document.';
        return;
      }
      form.saving = true;
      try {
        const total = parsed.value.docs.length;
        const result = await insertDocuments(ctx.name, parsed.value.docs, total === 1 ? 'Insert document' : `Insert ${total} documents`);
        reportInsert(result, total);
        if (result.stored > 0) {
          emit('update:open', false);
          ctx.refresh();
        }
      } catch (err) {
        form.error = err.message || 'The documents could not be inserted.';
      } finally {
        form.saving = false;
      }
    };

    return { form, parsed, defaults, submit, pluralize };
  },
  template: `
    <app-dialog :open="open" variant="drawer" labelledby="insert-title" @update:open="$emit('update:open', $event)">
      <div class="drawer-head">
        <div class="stack-sm">
          <h2 id="insert-title" class="drawer-title">Insert documents</h2>
          <p class="small muted">One JSON object, an array of objects, or one object per line.</p>
        </div>
        <button type="button" class="btn btn-ghost btn-sm btn-icon" aria-label="Close" @click="$emit('update:open', false)"><icon name="x" /></button>
      </div>
      <div class="drawer-body">
        <label for="insert-json" class="sr-only">Documents</label>
        <textarea id="insert-json" v-model="form.text" class="textarea" rows="18" spellcheck="false"
          @keydown.ctrl.enter.prevent="submit" @keydown.meta.enter.prevent="submit"></textarea>
        <p v-if="parsed.error" class="msg-error">{{ parsed.error }}</p>
        <p v-else-if="parsed.docs.length" class="small muted">{{ pluralize(parsed.docs.length, 'document') }} ready to insert.</p>
        <div v-if="Object.keys(defaults).length" class="panel small">
          <span class="strong">Defaults of this collection</span>
          <span class="muted">Missing fields are filled in on insert:</span>
          <code class="tiny break">{{ JSON.stringify(defaults) }}</code>
        </div>
        <p v-if="form.error && form.error !== parsed.error" class="msg-error">{{ form.error }}</p>
      </div>
      <div class="drawer-foot">
        <span class="hint" style="margin:0"><kbd>Ctrl</kbd> + <kbd>Enter</kbd> inserts</span>
        <div class="row">
          <button type="button" class="btn" @click="$emit('update:open', false)">Cancel</button>
          <button type="button" class="btn btn-primary" :disabled="form.saving || !parsed.docs.length || !!parsed.error" @click="submit">
            {{ form.saving ? 'Inserting…' : parsed.docs.length > 1 ? 'Insert ' + parsed.docs.length + ' documents' : 'Insert document' }}
          </button>
        </div>
      </div>
    </app-dialog>`,
};

// Converts "42" to 42 and "true"/"false" to booleans. Other text is kept.
const convertValue = (value) => {
  if (value === 'true') return true;
  if (value === 'false') return false;
  if (/^-?(0|[1-9]\d*)(\.\d+)?([eE][+-]?\d+)?$/.test(value)) return Number(value);
  return value;
};

const FORMATS = {
  csv: {
    title: 'Import CSV',
    intro: 'Each row becomes a document. The first row holds the field names.',
    accept: '.csv,text/csv,text/plain',
  },
  jsonl: {
    title: 'Import JSONL',
    intro: 'One JSON document per line (JSON Lines / NDJSON). A JSON array of documents also works.',
    accept: '.jsonl,.ndjson,.json,application/x-ndjson,application/json,text/plain',
  },
};

export const ImportDrawer = {
  name: 'ImportDrawer',
  components: { AppDialog, Icon },
  props: { format: { type: String, default: null } }, // csv | jsonl, null when closed
  emits: ['update:format'],
  setup(props, { emit }) {
    const ctx = inject(COLLECTION_KEY);
    const form = reactive({ file: null, text: '', delimiter: ',', trim: true, convert: false, importing: false, progress: 0, error: '', inputKey: 0 });

    watch(
      () => props.format,
      (format) => {
        if (format) Object.assign(form, { file: null, text: '', importing: false, progress: 0, error: '', inputKey: form.inputKey + 1 });
      },
    );

    const info = computed(() => FORMATS[props.format] || FORMATS.csv);

    const result = computed(() => {
      if (!form.text) return { docs: [], error: '' };
      if (props.format === 'jsonl') return parseDocuments(form.text);
      try {
        let docs = csvToDocuments(form.text, form.delimiter, form.trim);
        if (form.convert) docs = docs.map((doc) => Object.fromEntries(Object.entries(doc).map(([k, v]) => [k, convertValue(v)])));
        return { docs, error: '' };
      } catch (err) {
        return { docs: [], error: err.message };
      }
    });
    const preview = computed(() => result.value.docs.slice(0, 5));
    const columns = computed(() => {
      const set = new Set();
      preview.value.forEach((doc) => Object.keys(doc).forEach((k) => set.add(k)));
      return [...set];
    });

    const onFile = async (event) => {
      form.error = '';
      form.file = event.target.files?.[0] || null;
      form.text = '';
      if (!form.file) return;
      try {
        form.text = await form.file.text();
        if (props.format === 'csv') {
          const detected = detectCsvDelimiter(form.text.slice(0, 4096));
          if (detected) form.delimiter = detected;
        }
      } catch (err) {
        form.error = 'The file could not be read.';
      }
    };

    const close = () => emit('update:format', null);

    const submit = async () => {
      const docs = result.value.docs;
      if (!docs.length) return;
      form.importing = true;
      form.progress = 0;
      form.error = '';
      try {
        const outcome = await insertInBatches(ctx.name, docs, `Import ${form.file?.name || info.value.title}`, (stored) => {
          form.progress = stored;
        });
        reportInsert(outcome, docs.length);
        if (outcome.stored > 0) {
          close();
          ctx.refresh();
        } else {
          form.error = outcome.reason || 'The first document was rejected.';
        }
      } catch (err) {
        form.error = err.message || 'The file could not be imported.';
      } finally {
        form.importing = false;
      }
    };

    const cellClass = (value) => (typeof value === 'number' ? 'cell cell-number' : typeof value === 'boolean' ? 'cell cell-bool' : value !== null && typeof value === 'object' ? 'cell cell-json' : value === undefined ? 'cell cell-missing' : 'cell cell-string');
    const cell = (value) => (value === undefined ? '—' : truncate(typeof value === 'string' ? value : JSON.stringify(value), 40));

    return { form, info, result, columns, preview, onFile, close, submit, cell, cellClass, pluralize, formatNumber, delimiters: CSV_DELIMITERS };
  },
  template: `
    <app-dialog :open="!!format" variant="drawer" labelledby="import-title" @update:open="!$event && close()">
      <div class="drawer-head">
        <div class="stack-sm">
          <h2 id="import-title" class="drawer-title">{{ info.title }}</h2>
          <p class="small muted">{{ info.intro }}</p>
        </div>
        <button type="button" class="btn btn-ghost btn-sm btn-icon" aria-label="Close" @click="close"><icon name="x" /></button>
      </div>
      <div class="drawer-body">
        <div>
          <label for="import-file" class="label">File</label>
          <input id="import-file" :key="form.inputKey" type="file" :accept="info.accept" class="input" @change="onFile">
        </div>
        <div v-if="format === 'csv'" class="fields">
          <div>
            <label for="csv-delimiter" class="label">Delimiter</label>
            <select id="csv-delimiter" v-model="form.delimiter" class="select">
              <option v-for="d in delimiters" :key="d.label" :value="d.value">{{ d.label }}</option>
            </select>
          </div>
          <div class="stack-sm" style="align-content:end">
            <label class="check"><input v-model="form.trim" type="checkbox"> Trim whitespace</label>
            <label class="check"><input v-model="form.convert" type="checkbox"> Detect numbers and true/false</label>
          </div>
        </div>
        <p v-if="result.error" class="msg-error">{{ result.error }}</p>
        <template v-else-if="result.docs.length">
          <div class="row between">
            <span class="strong">Preview</span>
            <span class="small muted">{{ pluralize(result.docs.length, 'document') }}</span>
          </div>
          <div class="table-wrap card" style="box-shadow:none">
            <table class="data-table">
              <thead><tr><th v-for="c in columns" :key="c">{{ c }}</th></tr></thead>
              <tbody>
                <tr v-for="(doc, i) in preview" :key="i" style="cursor:default">
                  <td v-for="c in columns" :key="c"><span :class="cellClass(doc[c])">{{ cell(doc[c]) }}</span></td>
                </tr>
              </tbody>
            </table>
          </div>
          <p v-if="result.docs.length > preview.length" class="tiny faint">Showing the first {{ preview.length }} documents.</p>
        </template>
        <p v-if="form.importing" class="small muted" role="status">Imported {{ formatNumber(form.progress) }} of {{ formatNumber(result.docs.length) }}…</p>
        <p v-if="form.error" class="msg-error">{{ form.error }}</p>
      </div>
      <div class="drawer-foot">
        <span></span>
        <div class="row">
          <button type="button" class="btn" :disabled="form.importing" @click="close">Cancel</button>
          <button type="button" class="btn btn-primary" :disabled="form.importing || !result.docs.length" @click="submit">
            {{ form.importing ? 'Importing…' : result.docs.length ? 'Import ' + formatNumber(result.docs.length) + ' documents' : 'Import' }}
          </button>
        </div>
      </div>
    </app-dialog>`,
};
