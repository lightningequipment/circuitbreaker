import { forwardRef } from 'react';
import { Box, IconButton, Typography } from '@mui/material';

import { closeSnackbar, CustomContentProps, SnackbarContent } from 'notistack';

const ErrorSnackbar = forwardRef<HTMLDivElement, CustomContentProps>(
  ({ message, id }, ref) => (
    <SnackbarContent ref={ref}>
      <Box
        sx={{
          display: 'flex',
          overflow: 'hidden',
          width: { xs: '100%', md: '325px' },
          height: '52px',
        }}
      >
        <Box
          sx={{
            backgroundColor: 'error.main',
            p: 4,
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            borderTopLeftRadius: '8px',
            borderBottomLeftRadius: '8px',
            '> img': { display: 'block', width: '20px', height: '20px' },
          }}
        >
          <img src="/icons/exclamation.svg" alt="error" />
        </Box>
        <Box
          sx={{
            display: 'flex',
            flex: 1,
            p: 4,
            backgroundColor: 'grey.50',
            border: '1px solid',
            borderLeft: 'none',
            borderColor: 'grey.400',
            borderTopRightRadius: '8px',
            borderBottomRightRadius: '8px',
            alignItems: 'center',
          }}
        >
          <Box sx={{ flex: 1 }}>
            <Typography>{message}</Typography>
          </Box>
          <Box sx={{ pl: 4 }}>
            <IconButton type="button" onClick={() => closeSnackbar(id)}>
              <img src="/icons/close.svg" alt="close" />
            </IconButton>
          </Box>
        </Box>
      </Box>
    </SnackbarContent>
  )
);

export default ErrorSnackbar;
