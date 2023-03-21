import Document, { Html, Head, Main, NextScript } from 'next/document';
import { loaderStyles } from 'splashScreen';

class MyDocument extends Document {
  render() {
    return (
      <Html>
        <Head>
          {/* <script type="text/javascript" src="/config.js" /> */}
          <link
            rel="shortcut icon"
            href="/images/favicon.ico"
            type="image/x-icon"
          />
          <link
            rel="preload"
            as="image"
            href="/images/circuitbreaker-logo.svg"
          />
          <style>{loaderStyles}</style>
        </Head>
        <body>
          <div id="globalLoader">
            <img
              className="globalLoaderSpinner"
              src="/images/circuitbreaker-logo.svg"
              height="40"
              width="40"
              alt="loader"
            />
          </div>
          <Main />
          <NextScript />
        </body>
      </Html>
    );
  }
}

export default MyDocument;
