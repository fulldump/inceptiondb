// Indexes tab: list, create and delete indexes.

import { api, collectionUrl, confirmDanger, loadCollections, notify } from '../store.js';
import { COLLECTION_KEY, QUERY_KEY } from '../query.js';
import { formatBytes } from '../format.js';
import { Icon } from '../icons.js';
import { AppDialog } from './common.js';

const { computed, inject, reactive, watch } = Vue;

export const INDEX_TYPES = {
  map: { label: 'Map', desc: 'Exact lookup of one value, like an email. Unique.', fieldsLabel: 'Field', placeholder: 'email' },
  btree: { label: 'B-Tree', desc: 'Sorted ranges and ordering over one or more fields.', fieldsLabel: 'Fields', placeholder: 'lastName, -age' },
  fts: { label: 'Full text', desc: 'Search documents by the words of a text field.', fieldsLabel: 'Field', placeholder: 'description' },
  pk: { label: 'Primary key', desc: 'Unique key over one or more fields, checked on every write.', fieldsLabel: 'Fields', placeholder: 'id' },
};

const splitList = (text) => text.split(',').map((s) => s.trim()).filter(Boolean);

const fieldsOf = (index) => {
  if (index.type === 'btree') return (index.fields || []).join(', ');
  if (index.type === 'pk') return (index.paths || []).map((p) => p.join('.')).join(', ');
  return index.field || '';
};

const NewIndexDialog = {
  name: 'NewIndexDialog',
  components: { AppDialog },
  props: { open: { type: Boolean, default: false } },
  emits: ['update:open', 'created'],
  setup(props, { emit }) {
    const ctx = inject(COLLECTION_KEY);
    const form = reactive({ type: 'map', name: '', fields: '', sparse: false, unique: false, submitting: false, error: '' });

    // Suggest a name from the fields until the user types one.
    let nameTouched = false;
    watch(
      () => props.open,
      (open) => {
        if (!open) return;
        nameTouched = false;
        Object.assign(form, { type: 'map', name: '', fields: '', sparse: false, unique: false, submitting: false, error: '' });
      },
    );
    watch(
      () => [form.fields, form.type],
      () => {
        if (nameTouched) return;
        const base = splitList(form.fields).map((f) => f.replace(/^-/, '').replace(/\./g, '_')).join('_');
        form.name = base ? `${form.type === 'pk' ? 'pk' : 'by'}_${base}` : '';
      },
    );
    const onName = () => {
      nameTouched = form.name !== '';
    };

    const submit = async () => {
      form.error = '';
      const name = form.name.trim();
      const fields = splitList(form.fields);
      if (!fields.length) {
        form.error = 'Add at least one field.';
        return;
      }
      if (!name) {
        form.error = 'Give the index a name.';
        return;
      }
      if ((form.type === 'map' || form.type === 'fts') && fields.length > 1) {
        form.error = `A ${INDEX_TYPES[form.type].label} index covers a single field.`;
        return;
      }
      const payload = { name, type: form.type };
      if (form.type === 'map') Object.assign(payload, { field: fields[0], sparse: form.sparse });
      if (form.type === 'fts') payload.field = fields[0];
      if (form.type === 'btree') Object.assign(payload, { fields, sparse: form.sparse, unique: form.unique });
      if (form.type === 'pk') payload.paths = fields.map((f) => f.split('.'));
      form.submitting = true;
      try {
        await api(`Create index ${name}`, 'POST', collectionUrl(ctx.name, 'createIndex'), { body: payload });
        notify('success', `Index "${name}" created.`);
        emit('update:open', false);
        emit('created', name);
      } catch (err) {
        form.error = err.message || 'The index could not be created.';
      } finally {
        form.submitting = false;
      }
    };

    return { form, types: INDEX_TYPES, submit, onName };
  },
  template: `
    <app-dialog :open="open" wide labelledby="new-index-title" @update:open="$emit('update:open', $event)">
      <form @submit.prevent="submit">
        <div class="modal-head">
          <p id="new-index-title" class="modal-title">New index</p>
          <p class="small muted">Indexes make queries faster. Existing documents are indexed when you create it.</p>
        </div>
        <div class="modal-body">
          <div class="choice-grid" role="radiogroup" aria-label="Index type">
            <button v-for="(t, key) in types" :key="key" type="button" class="choice" role="radio"
              :aria-checked="form.type === key" :aria-pressed="form.type === key" @click="form.type = key">
              <span class="title">{{ t.label }}</span>
              <span class="desc">{{ t.desc }}</span>
            </button>
          </div>
          <div class="fields">
            <div>
              <label for="index-fields" class="label">{{ types[form.type].fieldsLabel }}</label>
              <input id="index-fields" v-model="form.fields" class="input mono" :placeholder="types[form.type].placeholder" autocomplete="off">
              <p class="hint" v-if="form.type === 'btree'">Comma separated. Prefix with "-" for descending order.</p>
              <p class="hint" v-else-if="form.type === 'pk'">Comma separated for a composite key; dots for nested fields.</p>
            </div>
            <div>
              <label for="index-name" class="label">Name</label>
              <input id="index-name" v-model="form.name" class="input" placeholder="by_email" autocomplete="off" @input="onName">
            </div>
          </div>
          <div v-if="form.type === 'map' || form.type === 'btree'" class="stack-sm">
            <label class="check"><input v-model="form.sparse" type="checkbox"> Sparse: skip documents that lack the field (otherwise they are rejected)</label>
            <label v-if="form.type === 'btree'" class="check"><input v-model="form.unique" type="checkbox"> Unique: reject documents with a repeated combination</label>
          </div>
          <p v-if="form.error" class="msg-error">{{ form.error }}</p>
        </div>
        <div class="modal-foot">
          <button type="button" class="btn" @click="$emit('update:open', false)">Cancel</button>
          <button type="submit" class="btn btn-primary" :disabled="form.submitting">{{ form.submitting ? 'Creating…' : 'Create index' }}</button>
        </div>
      </form>
    </app-dialog>`,
};

export const IndexesTab = {
  name: 'IndexesTab',
  components: { Icon, NewIndexDialog },
  setup() {
    const query = inject(QUERY_KEY);
    const ctx = inject(COLLECTION_KEY);
    const { state } = query;
    const dialog = reactive({ open: false });

    const sizeOf = (index) => {
      const value = ctx.metrics.data?.[`index.${index.name}`];
      return typeof value === 'number' && value >= 0 ? formatBytes(value) : '—';
    };
    const isUnique = (index) => index.type === 'map' || index.type === 'pk' || !!index.unique;

    const onCreated = async () => {
      await Promise.all([query.loadIndexes(), loadCollections(), ctx.refreshMetrics()]);
    };

    const remove = async (index) => {
      if (!(await confirmDanger(`Delete index "${index.name}"?`, 'Queries that use it will fall back to scanning the collection.', 'Delete index'))) return;
      try {
        await api(`Delete index ${index.name}`, 'POST', collectionUrl(ctx.name, 'dropIndex'), { body: { name: index.name } });
        if (state.indexName === index.name) state.indexName = '';
        notify('success', `Index "${index.name}" deleted.`);
        await onCreated();
      } catch (err) {
        notify('error', err.message || 'The index could not be deleted.');
      }
    };

    return { state, dialog, types: INDEX_TYPES, fieldsOf, sizeOf, isUnique, onCreated, remove };
  },
  template: `
    <section class="card">
      <div class="card-head">
        <div>
          <h2 class="card-title">Indexes</h2>
          <p class="card-sub">Unique indexes are checked on every write; the others are updated in the background.</p>
        </div>
        <button type="button" class="btn btn-primary" @click="dialog.open = true"><icon name="plus" /> New index</button>
      </div>
      <p v-if="state.indexesError" class="msg-error" style="padding:16px 20px">{{ state.indexesError }}</p>
      <div v-else-if="!state.indexes.length && !state.indexesLoading" class="empty">
        <div class="empty-icon"><icon name="zap" :size="22" /></div>
        <h3>No indexes</h3>
        <p>Queries scan every document. Add an index on the fields you filter or sort by.</p>
        <button type="button" class="btn btn-primary" @click="dialog.open = true"><icon name="plus" /> Create an index</button>
      </div>
      <div v-else class="table-wrap">
        <table class="data-table">
          <thead>
            <tr><th>Name</th><th>Type</th><th>Fields</th><th>Properties</th><th>Memory</th><th></th></tr>
          </thead>
          <tbody>
            <tr v-for="index in state.indexes" :key="index.name" style="cursor:default">
              <td class="strong">{{ index.name }}</td>
              <td><span class="badge">{{ (types[index.type] || {}).label || index.type }}</span></td>
              <td><span class="mono">{{ fieldsOf(index) }}</span></td>
              <td>
                <div class="row wrap">
                  <span v-if="isUnique(index)" class="badge badge-success">Unique</span>
                  <span v-if="index.sparse" class="badge">Sparse</span>
                </div>
              </td>
              <td class="mono muted">{{ sizeOf(index) }}</td>
              <td style="text-align:right"><button type="button" class="btn btn-sm btn-danger-soft" @click="remove(index)">Delete</button></td>
            </tr>
          </tbody>
        </table>
      </div>
      <new-index-dialog v-model:open="dialog.open" @created="onCreated" />
    </section>`,
};
