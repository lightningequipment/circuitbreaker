import { useTranslation } from 'react-i18next';
import { Box, Typography } from '@mui/material';

import { FOOTER_HEIGHT } from 'constant';

const Footer = () => {
  const { t } = useTranslation();

  return (
    <Box
      sx={{
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        height: FOOTER_HEIGHT,
      }}
    >
      <Typography sx={{ textAlign: 'center', color: 'grey.700' }}>
        {t('footer')}
      </Typography>
    </Box>
  );
};

export default Footer;
