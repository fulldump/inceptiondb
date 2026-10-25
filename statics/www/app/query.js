// Query state of the open collection, shared by its components through
// provide/inject (see CollectionView).

import { api, collectionUrl, parseNdjson } from './store.js';
import { DEFAULT_LIMIT, EMPTY_QUERY } from './route.js';
import { parseLooseValue, toNonNegativeInteger } from './format.js';

const { reactive, computed, watch } = Vue;

export const QUERYABLE_INDEX_TYPES = ['map', 'btree', 'fts'];

export const QUERY_KEY = Symbol('query');

// Context of the open collection (name, metrics, panels), see CollectionPage.
export const COLLECTION_KEY = Symbol('collection');

export function createQuery(collectionName, { route, onNavigate }) {
  const state = reactive({
    filterText: route.query || EMPTY_QUERY,
    filterError: '',
    indexName: '',
    mapValue: '',
    ftsMatch: '',
    reverse: false,
    rangeFrom: {},
    rangeTo: {},
    skip: route.skip || 0,
    limit: route.limit ?? DEFAULT_LIMIT,
    appliedSkip: route.skip || 0,
    appliedLimit: route.limit ?? DEFAULT_LIMIT,
    rows: [],
    loading: false,
    error: '',
    elapsedMs: null,
    lastPayload: null,
    indexes: [],
    indexesLoading: false,
    indexesError: '',
  });

  const activeIndex = computed(() => state.indexes.find((idx) => idx.name === state.indexName) || null);
  const queryableIndexes = computed(() => state.indexes.filter((idx) => QUERYABLE_INDEX_TYPES.includes(idx.type)));

  watch(activeIndex, (index) => {
    state.mapValue = '';
    state.ftsMatch = '';
    state.reverse = false;
    state.rangeFrom = {};
    state.rangeTo = {};
    (index?.type === 'btree' ? index.fields || [] : []).forEach((field) => {
      state.rangeFrom[field] = '';
      state.rangeTo[field] = '';
    });
  });

  // btreeBound keeps the filled fields; the server completes missing ones.
  const btreeBound = (values, fields) => {
    const bound = {};
    fields.forEach((field) => {
      const clean = field.replace(/^-/, '');
      if (String(values[field] ?? '').trim() !== '') bound[clean] = parseLooseValue(values[field]);
    });
    return Object.keys(bound).length > 0 ? bound : undefined;
  };

  // buildPayload returns the find request body, or throws with a user message.
  const buildPayload = () => {
    const raw = state.filterText.trim();
    let filter = {};
    if (raw) {
      try {
        filter = JSON.parse(raw);
      } catch (err) {
        state.filterError = `Invalid JSON: ${err.message}`;
        throw new Error('The filter must be valid JSON.');
      }
    }
    state.filterError = '';
    const payload = {
      filter,
      skip: toNonNegativeInteger(state.skip, 0),
      limit: toNonNegativeInteger(state.limit, 0),
    };
    const index = activeIndex.value;
    if (index) {
      payload.index = index.name;
      if (index.type === 'map') {
        if (!state.mapValue) throw new Error('You must provide a value for the selected index.');
        payload.value = state.mapValue;
      } else if (index.type === 'btree') {
        payload.reverse = !!state.reverse;
        const from = btreeBound(state.rangeFrom, index.fields || []);
        const to = btreeBound(state.rangeTo, index.fields || []);
        if (from) payload.from = from;
        if (to) payload.to = to;
      } else if (index.type === 'fts') {
        if (!state.ftsMatch) throw new Error('You must provide search terms for the selected index.');
        payload.match = state.ftsMatch;
      }
    }
    return payload;
  };

  const run = async () => {
    state.error = '';
    let payload;
    try {
      payload = buildPayload();
    } catch (err) {
      state.error = err.message;
      return;
    }
    state.loading = true;
    const started = performance.now();
    try {
      const { data } = await api('Query documents', 'POST', collectionUrl(collectionName, 'find'), { body: payload, text: true });
      state.rows = parseNdjson(data);
      state.elapsedMs = Math.round(performance.now() - started);
      state.skip = payload.skip;
      state.appliedSkip = payload.skip;
      state.appliedLimit = payload.limit;
      state.lastPayload = payload;
      onNavigate({ collection: collectionName, skip: payload.skip, limit: payload.limit, query: state.filterText });
    } catch (err) {
      state.error = err.message || 'Failed to fetch the collection.';
    } finally {
      state.loading = false;
    }
  };

  const applyRoute = (next) => {
    state.filterText = next.query || EMPTY_QUERY;
    state.filterError = '';
    state.skip = next.skip;
    state.limit = next.limit;
    return run();
  };

  const loadIndexes = async () => {
    state.indexesLoading = true;
    state.indexesError = '';
    try {
      const { data } = await api('List indexes', 'POST', collectionUrl(collectionName, 'listIndexes'), { background: true });
      state.indexes = Array.isArray(data) ? data : [];
      if (!state.indexes.some((idx) => idx.name === state.indexName)) state.indexName = '';
    } catch (err) {
      state.indexesError = err.message || 'Failed to load indexes.';
    } finally {
      state.indexesLoading = false;
    }
  };

  const disablePrev = computed(() => toNonNegativeInteger(state.appliedSkip, 0) <= 0);
  const disableNext = computed(() => {
    const limit = toNonNegativeInteger(state.appliedLimit, 0);
    return limit === 0 || state.rows.length < limit;
  });

  const nextPage = () => {
    if (disableNext.value) return;
    state.skip = state.appliedSkip + state.appliedLimit;
    run();
  };

  const prevPage = () => {
    if (disablePrev.value) return;
    state.skip = Math.max(0, state.appliedSkip - state.appliedLimit);
    run();
  };

  const commitSkip = () => {
    state.skip = toNonNegativeInteger(state.skip, 0);
    if (state.skip !== state.appliedSkip) run();
  };

  // Changing the page size keeps the current position (skip).
  const commitLimit = () => {
    state.limit = toNonNegativeInteger(state.limit, DEFAULT_LIMIT);
    if (state.limit !== state.appliedLimit) run();
  };

  return {
    collectionName,
    state,
    activeIndex,
    queryableIndexes,
    disablePrev,
    disableNext,
    run,
    applyRoute,
    loadIndexes,
    nextPage,
    prevPage,
    commitSkip,
    commitLimit,
  };
}
