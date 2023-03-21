import Head from 'next/head';
import type { AppProps } from 'next/app';
import { createTheme, ThemeProvider } from '@mui/material/styles';
import 'normalize.css';

import 'global.css';
import { QueryClientProvider } from 'providers';
import { Fonts } from 'components';
import customTheme from 'theme';
import { I18nextProvider } from 'react-i18next';

import i18n from '../../i18n';

const theme = createTheme(customTheme);

const App = ({ Component, pageProps }: AppProps) => (
  <>
    <Head>
      {/* <meta name="viewport" content="initial-scale=1, width=device-width" /> */}
      <title>Circuit Breaker ⚡️</title>
      <meta name="description" content="Advanced Lightning Node protection" />
    </Head>
    <I18nextProvider i18n={i18n}>
      <ThemeProvider theme={theme}>
        <QueryClientProvider>
          <Fonts />
          <Component {...pageProps} />
        </QueryClientProvider>
      </ThemeProvider>
    </I18nextProvider>
  </>
);

export default App;
