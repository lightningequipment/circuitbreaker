import { css, Global } from '@emotion/react';

export interface FontsProps {
  fontDisplay?: 'auto' | 'block' | 'fallback' | 'optional' | 'swap';
}

const Fonts = ({ fontDisplay = 'auto' }: FontsProps) => (
  <Global
    styles={css`
      @font-face {
        font-family: 'Helvetica Neue';
        src: url('/fonts/HelveticaNeue.eot') format('embedded-opentype'),
          url('/fonts/HelveticaNeue.woff2') format('woff2'),
          url('/fonts/HelveticaNeue.woff') format('woff'),
          url('/fonts/HelveticaNeue.ttf') format('truetype');
        font-weight: 500;
        font-display: ${fontDisplay};
      }

      @font-face {
        font-family: 'Helvetica Neue';
        src: url('/fonts/HelveticaNeueBold.eot') format('embedded-opentype'),
          url('/fonts/HelveticaNeueBold.woff2') format('woff2'),
          url('/fonts/HelveticaNeueBold.ttf') format('truetype');
        font-weight: 700;
        font-display: ${fontDisplay};
      }
    `}
  />
);

export default Fonts;
