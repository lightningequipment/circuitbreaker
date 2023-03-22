import { useEffect } from 'react';
import { useTranslation } from 'react-i18next';
import { Box } from '@mui/material';

import { useLimits, useInfo } from 'hooks';
import { removeLoader } from 'splashScreen';

import { Header, NodeTable } from './parts';

const Home = () => {
  const { isSuccess: isLimitsSuccess } = useLimits();
  const { isSuccess: isInfoSuccess } = useInfo();
  const { ready } = useTranslation();

  const isSuccess = isLimitsSuccess && isInfoSuccess && ready;

  useEffect(() => {
    removeLoader();
  }, [isSuccess]);

  if (!isSuccess) return null;

  return (
    <Box
      sx={{
        height: '100dvh',
        px: { xs: 0, md: 5, xl: 10 },
        pb: 5,
      }}
    >
      <Header />
      <NodeTable />
    </Box>
  );
};

export default Home;
