import React, { createContext, useContext, useEffect, useMemo, useState } from 'react';
import { Locale, TRANSLATIONS, TranslationKey } from './i18n/translations';

export { TRANSLATIONS };
export type { Locale, TranslationKey };

const LOCALE_STORAGE_KEY = 'likeable.locale';

type TranslationParams = Record<string, string | number>;

type I18nContextValue = {
  locale: Locale;
  setLocale: (locale: Locale) => void;
  t: (key: TranslationKey, params?: TranslationParams) => string;
};

const I18nContext = createContext<I18nContextValue | null>(null);

export function I18nProvider({ children }: { children: React.ReactNode }) {
  const [locale, setLocaleState] = useState<Locale>(() => initialLocale());
  const setLocale = (nextLocale: Locale) => {
    setLocaleState(nextLocale);
  };

  useEffect(() => {
    localStorage.setItem(LOCALE_STORAGE_KEY, locale);
    document.documentElement.lang = locale;
    document.querySelector<HTMLMetaElement>('meta[name="description"]')?.setAttribute('content', translate(locale, 'app.description'));
  }, [locale]);

  const value = useMemo<I18nContextValue>(() => ({
    locale,
    setLocale,
    t: (key, params) => translate(locale, key, params)
  }), [locale]);

  return <I18nContext.Provider value={value}>{children}</I18nContext.Provider>;
}

export function useI18n(): I18nContextValue {
  const context = useContext(I18nContext);
  if (!context) {
    throw new Error('useI18n must be used inside I18nProvider');
  }
  return context;
}

export function useDocumentTitle(title: string | null | undefined) {
  useEffect(() => {
    if (!title) return;
    document.title = title;
  }, [title]);
}

export function translate(locale: Locale, key: TranslationKey, params?: TranslationParams): string {
  const template = TRANSLATIONS[locale][key] ?? TRANSLATIONS.en[key] ?? key;
  if (!params) return template;
  return template.replace(/\{(\w+)\}/g, (match, name) => {
    const value = params[name];
    return value == null ? match : String(value);
  });
}

export function isLocale(value: string): value is Locale {
  return value === 'en' || value === 'uk';
}

export function resetCountdownLabels(t: (key: TranslationKey) => string) {
  return {
    fallback: t('duration.fiveHours'),
    lessThanMinute: t('duration.lessThanMinute'),
    day: t('duration.dayShort'),
    hour: t('duration.hourShort'),
    minute: t('duration.minuteShort')
  };
}

export function elapsedDurationLabels(t: (key: TranslationKey) => string) {
  return {
    minute: t('duration.minuteShort'),
    second: t('duration.secondShort')
  };
}

export function statusLabel(status: string | undefined, t: (key: TranslationKey) => string): string {
  switch ((status ?? '').trim().toLowerCase()) {
    case 'ready':
      return t('status.ready');
    case 'creating':
      return t('status.creating');
    case 'launching':
      return t('status.launching');
    case 'stopped':
      return t('status.stopped');
    case 'deleting':
      return t('status.deleting');
    case 'error':
      return t('status.error');
    case 'archived':
      return t('status.archived');
    case 'processing':
      return t('status.processing');
    default:
      return status || t('status.unknown');
  }
}

function initialLocale(): Locale {
  const stored = localStorage.getItem(LOCALE_STORAGE_KEY);
  if (stored && isLocale(stored)) return stored;
  const language = navigator.language.toLowerCase();
  return language.startsWith('uk') ? 'uk' : 'en';
}
