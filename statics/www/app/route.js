// Hash routing: #/collections/<name>[/<tab>]?skip=&limit=&query=

import { toNonNegativeInteger } from './format.js';

export const DEFAULT_LIMIT = 25;
export const EMPTY_QUERY = '{\n\n}';
export const TABS = ['documents', 'indexes', 'settings'];

const isMeaningfulQuery = (text) => {
  if (typeof text !== 'string' || !text.trim()) return false;
  try {
    const parsed = JSON.parse(text);
    if (parsed && typeof parsed === 'object' && !Array.isArray(parsed)) return Object.keys(parsed).length > 0;
    return true;
  } catch (err) {
    return true;
  }
};

export const parseRoute = (hash) => {
  const trimmed = String(hash || '').replace(/^#/, '');
  const [pathPart, searchPart = ''] = trimmed.split('?');
  const segments = pathPart.split('/').filter(Boolean);
  let collection = '';
  if (segments[0] === 'collections' && segments[1]) {
    try {
      collection = decodeURIComponent(segments[1]);
    } catch (err) {
      collection = segments[1];
    }
  }
  const tab = TABS.includes(segments[2]) ? segments[2] : 'documents';
  const params = new URLSearchParams(searchPart);
  return {
    collection,
    tab,
    skip: params.has('skip') ? toNonNegativeInteger(params.get('skip'), 0) : 0,
    limit: params.has('limit') ? toNonNegativeInteger(params.get('limit'), DEFAULT_LIMIT) : DEFAULT_LIMIT,
    query: params.has('query') && params.get('query').trim() ? params.get('query') : EMPTY_QUERY,
  };
};

export const buildRoute = ({ collection, tab = 'documents', skip = 0, limit = DEFAULT_LIMIT, query = '' }) => {
  if (!collection) return '#/';
  const path = `#/collections/${encodeURIComponent(collection)}${tab && tab !== 'documents' ? `/${tab}` : ''}`;
  const params = new URLSearchParams();
  if (skip > 0) params.set('skip', String(skip));
  if (limit !== DEFAULT_LIMIT || skip > 0) params.set('limit', String(limit));
  if (isMeaningfulQuery(query)) params.set('query', query);
  const search = params.toString();
  return search ? `${path}?${search}` : path;
};
