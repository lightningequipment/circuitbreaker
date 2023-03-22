import { Box, Typography } from '@mui/material';
import Image from 'next/image';

import { HEADER_HEIGHT_DESKTOP, HEADER_HEIGHT_MOBILE } from 'constant';
import { useInfo } from 'hooks';

import NodeInfo from './NodeInfo';
import DefaultLimits from './DefaultLimits';

const Header = () => {
  const { info } = useInfo();

  return (
    <Box
      sx={{
        display: 'flex',
        justifyContent: { xs: 'center', lg: 'space-between' },
        flexDirection: { xs: 'column', lg: 'row' },
        height: { xs: HEADER_HEIGHT_MOBILE, lg: HEADER_HEIGHT_DESKTOP },
        px: { xs: 4, md: 0 },
      }}
    >
      <Box sx={{ display: 'flex', alignItems: 'center', mb: { xs: 4, lg: 0 } }}>
        <Box
          sx={{
            mr: 4,
            img: {
              display: 'block',
            },
          }}
        >
          <Image
            src="/images/circuitbreaker-logo.svg"
            alt="Circuit Breaker"
            width={44}
            height={44}
          />
        </Box>

        <Box>
          <Typography variant="h3" sx={{ color: 'grey.50', mb: 1 }}>
            Circuit Breaker
          </Typography>
          <Box sx={{ display: 'flex', alignItems: 'center' }}>
            <Typography sx={{ color: 'grey.50' }}>{info.version}</Typography>
            <Box
              sx={{
                mx: 2,
                backgroundColor: 'grey.700',
                height: '4px',
                width: '4px',
                borderRadius: '50%',
              }}
            />
            <NodeInfo />
          </Box>
        </Box>
      </Box>
      <Typography
        sx={{
          color: 'grey.50',
          fontSize: '16px',
          mb: 2,
          display: { xs: 'inline-block', md: 'none' },
        }}
      >
        Default Limits
      </Typography>
      <DefaultLimits />
    </Box>
  );
};

export default Header;
