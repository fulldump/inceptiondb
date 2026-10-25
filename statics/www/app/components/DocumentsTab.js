// Documents tab: query bar, results (table or JSON) and pagination.

import { api, collectionUrl, confirmAction, notify, ui } from '../store.js';
import { COLLECTION_KEY, QUERY_KEY } from '../query.js';
import { conditionsFromFilter, filterFromConditions, filterText, OPERATORS } from '../filter.js';
import { documentId, formatNumber, pluralize, truncate } from '../format.js';
import { Icon } from '../icons.js';
import { JsonView } from './common.js';

const { computed, inject, nextTick, onBeforeUnmount, onMounted, reactive, ref, watch } = Vue;


// ---------- filter builder ----------

const FilterBar = {
  name: 'FilterBar',
  components: { Icon },
  props: { fields: { type: Array, default: () => [] } },
  setup() {
    const query = inject(QUERY_KEY);
    const { state } = query;
    const mode = ref('builder');
    const conditions = ref([]);
    let lastWritten = null;

    const parsedFilter = () => {
      const raw = state.filterText.trim();
      if (!raw) return {};
      try {
        return JSON.parse(raw);
      } catch (err) {
        return undefined;
      }
    };

    const representable = computed(() => {
      const parsed = parsedFilter();
      return parsed !== undefined && conditionsFromFilter(parsed) !== null;
    });

    // Load the conditions from the JSON text unless we just wrote it.
    const syncFromText = () => {
      if (state.filterText === lastWritten) return;
      const parsed = parsedFilter();
      const list = parsed === undefined ? null : conditionsFromFilter(parsed);
      if (list === null) {
        mode.value = 'json';
        return;
      }
      conditions.value = list;
    };
    watch(() => state.filterText, syncFromText, { immediate: true });

    const write = () => {
      lastWritten = filterText(filterFromConditions(conditions.value));
      state.filterText = lastWritten;
    };

    const add = async (field = '') => {
      conditions.value.push({ field, op: 'eq', value: '' });
      await nextTick();
      const inputs = document.querySelectorAll('.condition input.field');
      inputs[inputs.length - 1]?.focus();
    };
    const remove = (i) => {
      conditions.value.splice(i, 1);
      write();
      query.run();
    };
    const clear = () => {
      conditions.value = [];
      write();
      query.state.indexName = '';
      query.run();
    };
    const setMode = (next) => {
      if (next === 'builder' && !representable.value) return;
      mode.value = next;
      if (next === 'builder') {
        lastWritten = null;
        syncFromText();
      }
    };

    const hasFilter = computed(() => {
      const parsed = parsedFilter();
      return (parsed && Object.keys(parsed).length > 0) || !!state.indexName;
    });

    return { query, state, mode, conditions, representable, hasFilter, operators: OPERATORS, write, add, remove, clear, setMode };
  },
  template: `
    <div class="stack-sm">
      <div class="row wrap" style="align-items:flex-start">
        <div v-if="mode === 'builder'" class="conditions grow">
          <div v-for="(c, i) in conditions" :key="i" class="condition">
            <input v-model="c.field" class="field" list="field-suggestions" placeholder="field" aria-label="Field" @input="write" @keydown.enter.prevent="query.run">
            <select v-model="c.op" aria-label="Operator" @change="write">
              <option v-for="op in operators" :key="op.value" :value="op.value">{{ op.label }}</option>
            </select>
            <input v-model="c.value" :placeholder="c.op === 'in' || c.op === 'nin' ? 'a, b, c' : 'value'" aria-label="Value" @input="write" @keydown.enter.prevent="query.run">
            <button type="button" aria-label="Remove condition" @click="remove(i)"><icon name="x" :size="13" /></button>
          </div>
          <button type="button" class="btn btn-sm" @click="add()"><icon name="plus" /> {{ conditions.length ? 'Add condition' : 'Add filter' }}</button>
          <datalist id="field-suggestions"><option v-for="f in fields" :key="f" :value="f" /></datalist>
        </div>
        <div v-else class="grow">
          <label for="filter-json" class="sr-only">Filter (JSON)</label>
          <textarea id="filter-json" v-model="state.filterText" rows="4" class="textarea" :class="{ 'is-invalid': state.filterError }"
            spellcheck="false" placeholder='{"age": {"$gt": 30}}' @keydown.ctrl.enter.prevent="query.run" @keydown.meta.enter.prevent="query.run"></textarea>
          <p v-if="state.filterError" class="msg-error">{{ state.filterError }}</p>
          <p v-else class="hint">Operators: $eq $ne $gt $ge $lt $le $in $nin $contains $and $or. <kbd>Ctrl</kbd> + <kbd>Enter</kbd> runs the query.</p>
        </div>
        <div class="row">
          <button v-if="hasFilter" type="button" class="btn btn-ghost btn-sm" @click="clear">Clear</button>
          <div class="segmented" role="group" aria-label="Filter editor">
            <button type="button" :aria-pressed="mode === 'builder'" :disabled="!representable" :title="representable ? 'Visual filter' : 'This filter can only be edited as JSON'" @click="setMode('builder')">Builder</button>
            <button type="button" :aria-pressed="mode === 'json'" @click="setMode('json')">JSON</button>
          </div>
        </div>
      </div>
    </div>`,
};

// ---------- pager ----------

// Skip and limit are typed directly: jump to any offset, change the page
// size without losing the position.
const Pager = {
  name: 'Pager',
  components: { Icon },
  props: { id: { type: String, required: true } },
  setup() {
    const query = inject(QUERY_KEY);
    return { query, state: query.state };
  },
  template: `
    <div class="pager" role="group" aria-label="Pagination">
      <button type="button" aria-label="Previous page" :disabled="query.disablePrev.value || state.loading" @click="query.prevPage"><icon name="chevronLeft" :size="14" /><span class="pager-text">Prev</span></button>
      <label :for="id + '-skip'">skip
        <input :id="id + '-skip'" v-model.number="state.skip" type="number" min="0" aria-label="Skip" @change="query.commitSkip" @keyup.enter="query.commitSkip">
      </label>
      <label :for="id + '-limit'">limit
        <input :id="id + '-limit'" v-model.number="state.limit" type="number" min="0" aria-label="Limit" @change="query.commitLimit" @keyup.enter="query.commitLimit">
      </label>
      <button type="button" aria-label="Next page" :disabled="query.disableNext.value || state.loading" @click="query.nextPage"><span class="pager-text">Next</span><icon name="chevronRight" :size="14" /></button>
    </div>`,
};

// ---------- cell rendering ----------

const cellClass = (value, column) => {
  if (value === undefined) return 'cell cell-missing';
  if (column === 'id') return 'cell cell-id';
  if (value === null) return 'cell cell-null';
  if (typeof value === 'number') return 'cell cell-number';
  if (typeof value === 'boolean') return 'cell cell-bool';
  if (typeof value === 'object') return 'cell cell-json';
  return 'cell cell-string';
};

const cellText = (value) => {
  if (value === undefined) return '—';
  if (value === null) return 'null';
  if (typeof value === 'object') return truncate(JSON.stringify(value), 120);
  return truncate(String(value), 200);
};

// ---------- tab ----------

export const DocumentsTab = {
  name: 'DocumentsTab',
  components: { Icon, JsonView, FilterBar, Pager },
  setup() {
    const query = inject(QUERY_KEY);
    const ctx = inject(COLLECTION_KEY);
    const { state } = query;
    const goto = reactive({ id: '', loading: false });
    const exporting = ref(false);

    const columns = computed(() => {
      const counts = new Map();
      state.rows.forEach((row) => {
        if (row && typeof row === 'object' && !Array.isArray(row) && !('_raw' in row)) {
          Object.keys(row).forEach((k) => counts.set(k, (counts.get(k) || 0) + 1));
        }
      });
      return [...counts.keys()].sort((a, b) => {
        if (a === 'id') return -1;
        if (b === 'id') return 1;
        return counts.get(b) - counts.get(a) || a.localeCompare(b);
      });
    });

    const fieldSuggestions = computed(() => {
      const set = new Set(columns.value);
      state.indexes.forEach((idx) => {
        if (idx.field) set.add(idx.field);
        (idx.fields || []).forEach((f) => set.add(f.replace(/^-/, '')));
      });
      return [...set].sort();
    });

    // Suggest a map index when the filter compares its field by equality.
    const indexHint = computed(() => {
      if (state.indexName) return null;
      let filter;
      try {
        filter = JSON.parse(state.filterText || '{}');
      } catch (err) {
        return null;
      }
      for (const index of query.queryableIndexes.value) {
        if (index.type === 'map' && typeof filter[index.field] === 'string') return { index, value: filter[index.field] };
      }
      return null;
    });
    const useIndexHint = async () => {
      const { index, value } = indexHint.value;
      state.indexName = index.name;
      await nextTick();
      state.mapValue = value;
      query.run();
    };

    const isFiltered = computed(() => {
      try {
        const parsed = JSON.parse(state.filterText || '{}');
        return Object.keys(parsed).length > 0 || !!state.indexName;
      } catch (err) {
        return true;
      }
    });

    const range = computed(() => {
      const count = state.rows.length;
      if (!count) return '';
      const from = state.appliedSkip + 1;
      const to = state.appliedSkip + count;
      const total = ctx.collection.value?.total;
      if (!isFiltered.value && typeof total === 'number') return `${formatNumber(from)}–${formatNumber(to)} of ${formatNumber(total)}`;
      return `${formatNumber(from)}–${formatNumber(to)}`;
    });

    const openRow = (row) => ctx.openDocument(row);

    const gotoId = async () => {
      const id = goto.id.trim();
      if (!id) return;
      goto.loading = true;
      try {
        const url = `${collectionUrl(ctx.name)}/documents/${encodeURIComponent(id)}`;
        const { data } = await api('Get document by ID', 'GET', url);
        ctx.openDocument(data?.document || {});
        goto.id = '';
      } catch (err) {
        notify('error', err.status === 404 ? `No document with ID "${id}".` : err.message);
      } finally {
        goto.loading = false;
      }
    };

    const exportResults = async () => {
      const choice = await confirmAction({
        title: 'Export documents',
        message: 'Download the documents as JSON Lines (one document per line).',
        choices: [
          { value: 'page', label: `This page (${state.rows.length})` },
          { value: 'all', label: isFiltered.value ? 'All matching documents' : 'Whole collection', kind: 'primary' },
        ],
      });
      if (!choice) return;
      exporting.value = true;
      const safe = ctx.name.toLowerCase().replace(/[^a-z0-9-_]+/g, '_') || 'collection';
      const filename = `${safe}-${new Date().toISOString().slice(0, 19).replace(/[:T]/g, '-')}.jsonl`;
      try {
        let parts;
        let lines;
        if (choice === 'page') {
          parts = state.rows.map((row) => `${row.__rawLine ?? JSON.stringify(row)}\n`);
          lines = parts.length;
        } else {
          const body = { ...(state.lastPayload || { filter: {} }), skip: 0, limit: -1 };
          const response = await api('Export documents', 'POST', collectionUrl(ctx.name, 'find'), { body, stream: true });
          const reader = response.body.getReader();
          parts = [];
          lines = 0;
          for (;;) {
            const { done, value } = await reader.read();
            if (done) break;
            parts.push(value);
            for (let i = 0; i < value.length; i += 1) if (value[i] === 10) lines += 1;
          }
        }
        const url = URL.createObjectURL(new Blob(parts, { type: 'application/x-ndjson' }));
        const a = document.createElement('a');
        a.href = url;
        a.download = filename;
        document.body.appendChild(a);
        a.click();
        a.remove();
        setTimeout(() => URL.revokeObjectURL(url), 0);
        notify('success', `Exported ${pluralize(lines, 'document')} to ${filename}.`);
      } catch (err) {
        notify('error', err.message || 'The export failed.');
      } finally {
        exporting.value = false;
      }
    };

    // Horizontal scrollbar in the sticky footer, kept in sync with the table.
    // The table's own scrollbar sits below its last row, out of view on long lists.
    const tableWrap = ref(null);
    const hscroll = ref(null);
    // Sticky copy of the header: the real one cannot stick to the window
    // because the table sits in a horizontally scrolling container.
    const stickyHead = ref(null);
    const headWidths = ref([]);
    const headTableWidth = ref(0);
    const hscrollWidth = ref(0);
    const overflowing = ref(false);
    let syncing = false;

    const measure = () => {
      const wrap = tableWrap.value;
      const table = wrap?.firstElementChild;
      if (!wrap || !table) {
        overflowing.value = false;
        return;
      }
      hscrollWidth.value = table.scrollWidth;
      overflowing.value = table.scrollWidth > wrap.clientWidth + 1;
      headTableWidth.value = table.getBoundingClientRect().width;
      headWidths.value = [...table.querySelectorAll('thead th')].map((th) => th.getBoundingClientRect().width);
      if (hscroll.value) hscroll.value.scrollLeft = wrap.scrollLeft;
      if (stickyHead.value) stickyHead.value.scrollLeft = wrap.scrollLeft;
    };
    const follow = (from, to) => {
      if (stickyHead.value && from) stickyHead.value.scrollLeft = from.scrollLeft;
      if (syncing || !from || !to) return;
      syncing = true;
      to.scrollLeft = from.scrollLeft;
      requestAnimationFrame(() => {
        syncing = false;
      });
    };
    const onTableScroll = () => follow(tableWrap.value, hscroll.value);
    const onBarScroll = () => follow(hscroll.value, tableWrap.value);

    const resizer = new ResizeObserver(measure);
    watch(
      () => [state.rows, ui.resultsView, columns.value.length],
      async () => {
        await nextTick();
        resizer.disconnect();
        if (tableWrap.value) {
          resizer.observe(tableWrap.value);
          if (tableWrap.value.firstElementChild) resizer.observe(tableWrap.value.firstElementChild);
        }
        measure();
      },
      { immediate: true },
    );
    onBeforeUnmount(() => resizer.disconnect());

    // "/" focuses the filter, like most data tools.
    const onKey = (event) => {
      if (event.key !== '/' || ['INPUT', 'TEXTAREA', 'SELECT'].includes(document.activeElement?.tagName)) return;
      const target = document.querySelector('.condition input.field, #filter-json');
      if (target) {
        event.preventDefault();
        target.focus();
      }
    };
    onMounted(() => document.addEventListener('keydown', onKey));
    onBeforeUnmount(() => document.removeEventListener('keydown', onKey));

    return {
      tableWrap, hscroll, hscrollWidth, overflowing, onTableScroll, onBarScroll, stickyHead, headWidths, headTableWidth,
      query, state, ctx, ui, goto, exporting, columns, fieldSuggestions, indexHint, useIndexHint, isFiltered, range,
      openRow, gotoId, exportResults, cellClass, cellText, documentId, formatNumber,
    };
  },
  template: `
    <section class="card" aria-label="Documents">
      <form class="querybar" @submit.prevent="query.run">
        <filter-bar :fields="fieldSuggestions" />
        <div class="row wrap">
          <label for="query-index" class="tiny muted">Use index</label>
          <select id="query-index" v-model="state.indexName" class="select select-sm" style="width:auto">
            <option value="">None (full scan)</option>
            <option v-for="index in query.queryableIndexes.value" :key="index.name" :value="index.name">{{ index.name }} · {{ index.type }}</option>
          </select>
          <template v-if="query.activeIndex.value && query.activeIndex.value.type === 'map'">
            <label for="query-map-value" class="sr-only">Value of {{ query.activeIndex.value.field }}</label>
            <input id="query-map-value" v-model="state.mapValue" class="input input-sm" style="width:200px" :placeholder="query.activeIndex.value.field + ' ='">
          </template>
          <template v-if="query.activeIndex.value && query.activeIndex.value.type === 'fts'">
            <label for="query-fts" class="sr-only">Search terms</label>
            <input id="query-fts" v-model="state.ftsMatch" class="input input-sm" style="width:220px" placeholder="Words to search">
          </template>
          <template v-if="query.activeIndex.value && query.activeIndex.value.type === 'btree'">
            <span v-for="field in query.activeIndex.value.fields" :key="field" class="condition">
              <input v-model="state.rangeFrom[field]" :aria-label="field + ' from'" placeholder="from" style="width:80px">
              <span class="mono tiny" style="align-self:center">≤ {{ field.replace(/^-/, '') }} ≤</span>
              <input v-model="state.rangeTo[field]" :aria-label="field + ' to'" placeholder="to" style="width:80px">
            </span>
            <label class="check"><input v-model="state.reverse" type="checkbox"> Reverse</label>
          </template>
          <span class="grow"></span>
          <button type="submit" class="btn btn-primary btn-sm" :disabled="state.loading"><icon name="play" :size="13" /> {{ state.loading ? 'Running…' : 'Run' }}</button>
        </div>
        <div v-if="indexHint" class="index-hint">
          <icon name="zap" :size="13" />
          <span class="grow">The index <b>{{ indexHint.index.name }}</b> can answer this filter without scanning the collection.</span>
          <button type="button" class="btn btn-sm" @click="useIndexHint">Use index</button>
        </div>
      </form>

      <div class="results-head">
        <div class="row wrap small muted">
          <span v-if="state.loading">Running query…</span>
          <span v-else-if="state.error" class="msg-error">{{ state.error }}</span>
          <template v-else>
            <span v-if="state.rows.length"><b class="strong">{{ range }}</b> {{ isFiltered ? 'matching documents' : 'documents' }}</span>
            <span v-else-if="state.elapsedMs !== null"><b class="strong">0</b> documents</span>
            <span v-if="state.elapsedMs !== null" class="faint">· {{ state.elapsedMs }} ms</span>
          </template>
          <pager v-if="state.rows.length || state.appliedSkip > 0" id="pager-top" />
        </div>
        <div class="row wrap">
          <form class="row" @submit.prevent="gotoId">
            <label for="goto-id" class="sr-only">Open document by ID</label>
            <input id="goto-id" v-model="goto.id" class="input input-sm" style="width:170px" placeholder="Go to ID…" autocomplete="off">
          </form>
          <div class="segmented" role="group" aria-label="View">
            <button type="button" :aria-pressed="ui.resultsView === 'table'" @click="ui.resultsView = 'table'">Table</button>
            <button type="button" :aria-pressed="ui.resultsView === 'json'" @click="ui.resultsView = 'json'">JSON</button>
          </div>
          <button type="button" class="btn btn-sm" :disabled="exporting || !state.rows.length" @click="exportResults"><icon name="download" /> {{ exporting ? 'Exporting…' : 'Export' }}</button>
        </div>
      </div>

      <div v-if="!state.loading && !state.error && !state.rows.length" class="empty">
        <template v-if="isFiltered">
          <div class="empty-icon"><icon name="search" :size="22" /></div>
          <h3>No documents match</h3>
          <p>Nothing in this collection matches the current filter. Try a broader condition.</p>
        </template>
        <template v-else-if="state.appliedSkip > 0">
          <div class="empty-icon"><icon name="table" :size="22" /></div>
          <h3>No more documents</h3>
          <p>This page is past the end of the collection.</p>
          <button type="button" class="btn" @click="query.state.skip = 0; query.run()">Back to the first page</button>
        </template>
        <template v-else>
          <div class="empty-icon"><icon name="database" :size="22" /></div>
          <h3>This collection is empty</h3>
          <p>Insert JSON documents or import a CSV or JSONL file to get started.</p>
          <div class="row">
            <button type="button" class="btn" @click="ctx.openImport('csv')"><icon name="upload" /> Import CSV</button>
            <button type="button" class="btn" @click="ctx.openImport('jsonl')"><icon name="upload" /> Import JSONL</button>
            <button type="button" class="btn btn-primary" @click="ctx.openInsert"><icon name="plus" /> Insert documents</button>
          </div>
        </template>
      </div>

      <template v-else-if="state.rows.length && ui.resultsView === 'table'">
        <div ref="stickyHead" class="table-head-sticky" aria-hidden="true">
          <table class="data-table" :style="{ width: headTableWidth + 'px' }">
            <thead>
              <tr>
                <th class="col-index" :style="{ width: (headWidths[0] || 0) + 'px' }">#</th>
                <th v-for="(column, i) in columns" :key="column" :style="{ width: (headWidths[i + 1] || 0) + 'px' }">{{ column }}</th>
              </tr>
            </thead>
          </table>
        </div>
        <div ref="tableWrap" class="table-wrap table-wrap-synced" @scroll="onTableScroll">
          <table class="data-table">
            <thead class="thead-measure">
              <tr>
                <th scope="col" class="col-index">#</th>
                <th v-for="column in columns" :key="column" scope="col">{{ column }}</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="(row, idx) in state.rows" :key="documentId(row) ?? idx" tabindex="0"
                @click="openRow(row)" @keydown.enter="openRow(row)">
                <td class="col-index">{{ formatNumber(state.appliedSkip + idx + 1) }}</td>
                <td v-if="typeof row._raw === 'string'" :colspan="columns.length || 1"><span class="cell cell-json">{{ row._raw }}</span></td>
                <td v-else v-for="column in columns" :key="column"><span :class="cellClass(row[column], column)">{{ cellText(row[column]) }}</span></td>
              </tr>
            </tbody>
          </table>
        </div>
      </template>

      <div v-else-if="state.rows.length" class="doc-list">
        <article v-for="(row, idx) in state.rows" :key="documentId(row) ?? idx" class="doc-item" tabindex="0"
          @click="openRow(row)" @keydown.enter="openRow(row)">
          <span class="tiny faint">#{{ formatNumber(state.appliedSkip + idx + 1) }}</span>
          <pre v-if="typeof row._raw === 'string'" class="json">{{ row._raw }}</pre>
          <json-view v-else :value="row" />
        </article>
      </div>

      <div v-if="state.rows.length || state.appliedSkip > 0" class="results-foot">
        <div v-show="overflowing" ref="hscroll" class="hscroll" aria-hidden="true" @scroll="onBarScroll">
          <div :style="{ width: hscrollWidth + 'px' }"></div>
        </div>
        <span>{{ range }}</span>
        <pager id="pager-bottom" />
      </div>
    </section>`,
};
