import i18n from 'i18next';
import { initReactI18next } from 'react-i18next';

import Backend from 'i18next-http-backend';

i18n
  .use(Backend)
  .use(initReactI18next)
  .init({
    load: 'languageOnly',
    lng: 'en',
    fallbackLng: 'en',
    ns: 'common',
    defaultNS: 'common',
    react: {
      useSuspense: false,
    },
  });

export default i18n;
