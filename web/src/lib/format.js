// format.js — numeri e date nella lingua dell'utente.
//
// Il backend manda dati grezzi (peso, ripetizioni, timestamp) e la frase la
// compone qui. È lo stesso motivo per cui gli errori arrivano come codici: il
// server non sa in che lingua sta guardando l'utente. Prima i PR arrivavano già
// scritti — `value: "100.0 kg × 5"`, `when: "Today"` — e comparivano in inglese
// dentro un'app che parte in italiano.
//
// Si usa Intl invece di aggiungere chiavi di traduzione: le forme dei tempi
// relativi (oggi / ieri / 3 giorni fa / 2 settimane fa) e i separatori decimali
// (102,5 contro 102.5) li conosce già la piattaforma, plurali compresi.

import { derived } from 'svelte/store';
import { intlLocale, t } from './i18n/index.js';

const startOfDay = (d) => {
  const x = new Date(d);
  x.setHours(0, 0, 0, 0);
  return x;
};

// relativeDay(iso) → 'oggi' | 'ieri' | '3 giorni fa' | '2 settimane fa' | ...
//
// Conta i giorni di calendario, non le ore passate: un allenamento di ieri sera
// è "ieri" anche se sono trascorse solo venti ore. Il vecchio codice lato server
// usava le ore, quindi lo chiamava "Today".
export const relativeDay = derived(intlLocale, ($loc) => {
  const rtf = new Intl.RelativeTimeFormat($loc, { numeric: 'auto' });
  return (iso) => {
    if (!iso) return '';
    const then = new Date(iso);
    if (Number.isNaN(then.getTime())) return '';

    const days = Math.round((startOfDay(new Date()) - startOfDay(then)) / 86400000);
    if (days <= 0) return rtf.format(0, 'day'); // oggi (anche per date future per sicurezza)
    if (days < 7) return rtf.format(-days, 'day');
    if (days < 30) return rtf.format(-Math.round(days / 7), 'week');
    return rtf.format(-Math.round(days / 30), 'month');
  };
});

// weightReps(100, 5) → '100 kg × 5'  /  weightReps(102.5, 3) → '102,5 kg × 3' in it
export const weightReps = derived([intlLocale, t], ([$loc, $t]) => {
  const nf = new Intl.NumberFormat($loc, { maximumFractionDigits: 1 });
  return (weight, reps) => `${nf.format(weight ?? 0)} ${$t('common.kg')} × ${reps ?? 0}`;
});
