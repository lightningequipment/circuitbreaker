import React from 'react';
import Head from 'next/head';
import type { AppProps } from 'next/app';
import { createTheme, ThemeProvider } from '@mui/material/styles';
import { SnackbarProvider } from 'notistack';
import 'normalize.css';

import 'global.css';
import { QueryClientProvider } from 'providers';
import { Fonts, ErrorSnackbar } from 'components';
import customTheme from 'theme';
import { I18nextProvider } from 'react-i18next';

import i18n from '../../i18n';

const theme = createTheme(customTheme);

// Only render i18n provider on client side since it is a static webapp
const ClientI18nextProvider = ({ children }: React.PropsWithChildren<{}>) =>
  typeof window !== undefined ? (
    <I18nextProvider i18n={i18n}>{children}</I18nextProvider>
  ) : (
    <>{children}</>
  );

const App = ({ Component, pageProps }: AppProps) => (
  <>
    <Head>
      {/* <meta name="viewport" content="initial-scale=1, width=device-width" /> */}
      <title>Circuit Breaker ⚡️</title>
      <meta name="description" content="Advanced Lightning Node protection" />
    </Head>
    <ClientI18nextProvider>
      <ThemeProvider theme={theme}>
        <SnackbarProvider
          maxSnack={2}
          anchorOrigin={{ vertical: 'top', horizontal: 'center' }}
          autoHideDuration={6000}
          Components={{
            error: ErrorSnackbar,
          }}
        >
          <QueryClientProvider>
            <Fonts />
            <Component {...pageProps} />
          </QueryClientProvider>
        </SnackbarProvider>
      </ThemeProvider>
    </ClientI18nextProvider>
  </>
);

export default App;
