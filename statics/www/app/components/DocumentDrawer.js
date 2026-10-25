// Side panel to view, edit, copy and delete one document.

import { api, collectionUrl, confirmDanger, notify, parseNdjson } from '../store.js';
import { COLLECTION_KEY, QUERY_KEY } from '../query.js';
import { buildMergePatch } from '../patch.js';
import { documentId, prettyJson } from '../format.js';
import { Icon } from '../icons.js';
import { AppDialog, JsonView } from './common.js';

const { computed, inject, reactive, watch } = Vue;

export const DocumentDrawer = {
  name: 'DocumentDrawer',
  components: { AppDialog, Icon, JsonView },
  props: { document: { type: Object, default: null } },
  emits: ['update:document'],
  setup(props, { emit }) {
    const query = inject(QUERY_KEY);
    const ctx = inject(COLLECTION_KEY);
    const edit = reactive({ active: false, draft: '', error: '', saving: false });

    const id = computed(() => documentId(props.document));
    const position = computed(() => query.state.rows.findIndex((row) => row === props.document || (id.value !== null && documentId(row) === id.value)));

    watch(
      () => props.document,
      () => Object.assign(edit, { active: false, draft: '', error: '', saving: false }),
    );

    const close = () => emit('update:document', null);
    const show = (offset) => {
      const next = query.state.rows[position.value + offset];
      if (next) emit('update:document', next);
    };

    const copy = async () => {
      try {
        await navigator.clipboard.writeText(prettyJson(props.document));
        notify('success', 'Document copied to the clipboard.');
      } catch (err) {
        notify('error', 'The clipboard is not available in this browser.');
      }
    };

    const startEdit = () => Object.assign(edit, { active: true, draft: prettyJson(props.document), error: '' });

    const save = async () => {
      edit.error = '';
      let parsed;
      try {
        parsed = JSON.parse(edit.draft);
      } catch (err) {
        edit.error = `Invalid JSON: ${err.message}`;
        return;
      }
      if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) {
        edit.error = 'A document must be a JSON object.';
        return;
      }
      if (parsed.id !== props.document.id) {
        edit.error = 'The "id" field cannot be changed or removed.';
        return;
      }
      const patch = buildMergePatch(props.document, parsed);
      if (patch) delete patch.id;
      if (!patch || !Object.keys(patch).length) {
        edit.active = false;
        notify('info', 'No changes to save.');
        return;
      }
      edit.saving = true;
      try {
        const { data } = await api(`Update document ${id.value}`, 'POST', collectionUrl(ctx.name, 'patch'), {
          body: { filter: { id: props.document.id }, patch, limit: 1 },
          text: true,
        });
        const updated = parseNdjson(data)[0] || parsed;
        // Update the row in place so the table reflects the change.
        const row = query.state.rows[position.value];
        if (row) {
          Object.keys(row).forEach((key) => delete row[key]);
          Object.assign(row, updated);
        }
        edit.active = false;
        emit('update:document', row || updated);
        notify('success', `Document ${id.value} saved.`);
      } catch (err) {
        edit.error = err.message || 'The document could not be saved.';
      } finally {
        edit.saving = false;
      }
    };

    const remove = async () => {
      if (!(await confirmDanger(`Delete document ${id.value}?`, 'The document will be permanently deleted.', 'Delete document'))) return;
      try {
        await api(`Delete document ${id.value}`, 'POST', collectionUrl(ctx.name, 'remove'), { body: { filter: { id: props.document.id }, limit: 1 } });
        notify('success', `Document ${id.value} deleted.`);
        close();
        ctx.refresh();
      } catch (err) {
        notify('error', err.message || 'The document could not be deleted.');
      }
    };

    return { query, id, position, edit, close, show, copy, startEdit, save, remove };
  },
  template: `
    <app-dialog :open="!!document" variant="drawer" labelledby="document-title" @update:open="!$event && close()">
      <div class="drawer-head">
        <div class="stack-sm">
          <span class="tiny faint">Document{{ position >= 0 ? ' #' + (query.state.appliedSkip + position + 1) : '' }}</span>
          <h2 id="document-title" class="drawer-title mono break">{{ id ?? 'Without id' }}</h2>
        </div>
        <div class="row">
          <button type="button" class="btn btn-sm btn-icon" aria-label="Previous document" :disabled="position <= 0 || edit.active" @click="show(-1)"><icon name="chevronLeft" /></button>
          <button type="button" class="btn btn-sm btn-icon" aria-label="Next document" :disabled="position < 0 || position >= query.state.rows.length - 1 || edit.active" @click="show(1)"><icon name="chevronRight" /></button>
          <button type="button" class="btn btn-ghost btn-sm btn-icon" aria-label="Close" @click="close"><icon name="x" /></button>
        </div>
      </div>
      <div class="drawer-body">
        <template v-if="!edit.active">
          <json-view :value="document" />
          <p v-if="id === null" class="tiny faint">Documents without an "id" field cannot be edited or deleted from the console.</p>
        </template>
        <template v-else>
          <label for="document-editor" class="sr-only">Document JSON</label>
          <textarea id="document-editor" v-model="edit.draft" class="textarea" rows="22" spellcheck="false"
            @keydown.ctrl.enter.prevent="save" @keydown.meta.enter.prevent="save"></textarea>
          <p class="hint">Only the changed fields are sent (JSON merge patch). Remove a field to delete it. <kbd>Ctrl</kbd> + <kbd>Enter</kbd> saves.</p>
          <p v-if="edit.error" class="msg-error">{{ edit.error }}</p>
        </template>
      </div>
      <div class="drawer-foot">
        <template v-if="!edit.active">
          <button type="button" class="btn btn-danger-soft" :disabled="id === null" @click="remove"><icon name="trash" /> Delete</button>
          <div class="row">
            <button type="button" class="btn" @click="copy"><icon name="copy" /> Copy</button>
            <button type="button" class="btn btn-primary" :disabled="id === null" @click="startEdit"><icon name="edit" /> Edit</button>
          </div>
        </template>
        <template v-else>
          <span></span>
          <div class="row">
            <button type="button" class="btn" :disabled="edit.saving" @click="edit.active = false">Cancel</button>
            <button type="button" class="btn btn-primary" :disabled="edit.saving" @click="save">{{ edit.saving ? 'Saving…' : 'Save changes' }}</button>
          </div>
        </template>
      </div>
    </app-dialog>`,
};
