// i18n — Italian by default, English as the alternative.
//
// Hand-rolled rather than pulling in a library: two locales and ~100 strings
// don't justify a build step or a dependency, and this matches how the rest of
// the app is put together (see service/email.go, service/oauth_google.go).

import { derived, writable } from 'svelte/store';
import { it } from './it.js';
import { en } from './en.js';

const MESSAGES = { it, en };

export const LOCALES = ['it', 'en'];
export const DEFAULT_LOCALE = 'it';

const KEY = 'elite.locale';

// localStorage throws in private mode; the app is SPA-only but stay defensive,
// same as lib/api.js does for the session token.
function ls(fn, fallback = null) {
  try {
    return fn();
  } catch {
    return fallback;
  }
}

// Stored choice wins, then the browser's preference, then Italian. Anything
// unrecognised falls back rather than showing raw keys.
function initialLocale() {
  if (typeof window === 'undefined') return DEFAULT_LOCALE;
  const saved = ls(() => localStorage.getItem(KEY));
  if (LOCALES.includes(saved)) return saved;
  const nav = navigator.language?.slice(0, 2);
  return LOCALES.includes(nav) ? nav : DEFAULT_LOCALE;
}

export const locale = writable(initialLocale());

locale.subscribe((l) => {
  if (typeof document === 'undefined') return;
  ls(() => localStorage.setItem(KEY, l));
  // Keeps <html lang> honest — screen readers and hyphenation rely on it.
  document.documentElement.lang = l;
});

export function setLocale(l) {
  if (LOCALES.includes(l)) locale.set(l);
}

function lookup(messages, key) {
  return key.split('.').reduce((node, part) => (node == null ? undefined : node[part]), messages);
}

// Fills {placeholders}. Missing values are left visible rather than printed as
// "undefined", so a bad call is obvious instead of silently wrong.
function interpolate(str, vars) {
  if (!vars) return str;
  return str.replace(/\{(\w+)\}/g, (whole, name) => (name in vars ? String(vars[name]) : whole));
}

export function translate(loc, key, vars) {
  const messages = MESSAGES[loc] ?? MESSAGES[DEFAULT_LOCALE];
  let hit = lookup(messages, key) ?? lookup(en, key);
  // Backend error codes are open-ended — a new one shipped server-side must
  // not surface as the literal string "errors.some_new_code".
  if (typeof hit !== 'string' && key.startsWith('errors.')) {
    hit = lookup(messages, 'errors.unknown') ?? lookup(en, 'errors.unknown');
  }
  // Otherwise returning the key makes a missing string loud instead of blank.
  if (typeof hit !== 'string') return key;
  return interpolate(hit, vars);
}

/** Usage in markup: {$t('login.title')} or {$t('train.weeksLeft', { n: 4 })}. */
export const t = derived(locale, ($locale) => (key, vars) => translate($locale, key, vars));

/** Locale tag for Intl / toLocaleString — the app formats numbers and dates. */
export const intlLocale = derived(locale, ($locale) => ($locale === 'it' ? 'it-IT' : 'en-GB'));
