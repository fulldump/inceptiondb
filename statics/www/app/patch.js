// JSON merge patch (RFC 7386) between two versions of a document.

const isPlainObject = (value) => Object.prototype.toString.call(value) === '[object Object]';

export const deepEqual = (a, b) => {
  if (a === b) return true;
  if (typeof a !== typeof b || a === null || b === null) return false;
  if (Array.isArray(a)) {
    if (!Array.isArray(b) || a.length !== b.length) return false;
    return a.every((item, i) => deepEqual(item, b[i]));
  }
  if (isPlainObject(a)) {
    if (!isPlainObject(b)) return false;
    const keys = new Set([...Object.keys(a), ...Object.keys(b)]);
    for (const key of keys) {
      if (!deepEqual(a[key], b[key])) return false;
    }
    return true;
  }
  return false;
};

// buildMergePatch returns the patch that turns original into updated, or
// undefined when they are equal. Removed fields become null.
export const buildMergePatch = (original, updated) => {
  if (deepEqual(original, updated)) return undefined;
  if (!isPlainObject(original) || !isPlainObject(updated)) return updated;
  const patch = {};
  const keys = new Set([...Object.keys(original), ...Object.keys(updated)]);
  keys.forEach((key) => {
    if (!Object.prototype.hasOwnProperty.call(updated, key)) {
      patch[key] = null;
      return;
    }
    const child = buildMergePatch(original[key], updated[key]);
    if (child !== undefined) patch[key] = child;
  });
  return Object.keys(patch).length === 0 ? undefined : patch;
};
