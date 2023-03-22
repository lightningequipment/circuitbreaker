import { useState } from 'react';
import {
  Box,
  Typography,
  Drawer,
  List,
  ListItemButton,
  ListItemText,
  Divider,
  useMediaQuery,
} from '@mui/material';
import { ClickAwayListener } from '@mui/base';
import { useTranslation } from 'react-i18next';
import Image from 'next/image';
import { useTheme } from '@mui/material/styles';

import { SecondaryButton, Checkbox } from 'components';

interface ColumnsSettingProps {
  checkedSettings: { [K in ColumnId]: boolean };
  handleToggleSettings: (value: ColumnId) => void;
}

interface ColumnOptionProps {
  text: string;
  isChecked: boolean;
  onClick: () => void;
}

const ColumnOption = ({ text, isChecked, onClick }: ColumnOptionProps) => (
  <ListItemButton
    onClick={(e) => {
      e.preventDefault();
      onClick();
    }}
  >
    <ListItemText primary={text} sx={{ fontWeight: 'bold' }} />
    <Checkbox size="small" color="primary" checked={isChecked} />
  </ListItemButton>
);

const ColumnsSetting = ({
  checkedSettings,
  handleToggleSettings,
}: ColumnsSettingProps) => {
  const { t } = useTranslation('common', { keyPrefix: 'node-table' });

  const theme = useTheme();
  const isTablet = useMediaQuery(theme.breakpoints.up('md'));

  const [showSettings, setShowSettings] = useState(false);

  const ColumnSettingOptions = () => (
    <ClickAwayListener onClickAway={() => setShowSettings(false)}>
      <List>
        <Typography variant="h5" sx={{ px: 3, py: 1 }}>
          {t('counter1h.title')}
        </Typography>
        <ColumnOption
          text={t('counter1h.success')}
          isChecked={checkedSettings['counter1h.success']}
          onClick={() => {
            handleToggleSettings('counter1h.success');
          }}
        />
        <ColumnOption
          text={t('counter1h.fail')}
          isChecked={checkedSettings['counter1h.fail']}
          onClick={() => {
            handleToggleSettings('counter1h.fail');
          }}
        />
        <ColumnOption
          text={t('counter1h.reject')}
          isChecked={checkedSettings['counter1h.reject']}
          onClick={() => {
            handleToggleSettings('counter1h.reject');
          }}
        />
        <Divider sx={{ my: 1, mx: { xs: -4, md: 0 } }} />
        <Typography variant="h5" sx={{ px: 3, py: 1 }}>
          {t('counter24h.title')}
        </Typography>
        <ColumnOption
          text={t('counter24h.success')}
          isChecked={checkedSettings['counter24h.success']}
          onClick={() => {
            handleToggleSettings('counter24h.success');
          }}
        />
        <ColumnOption
          text={t('counter24h.fail')}
          isChecked={checkedSettings['counter24h.fail']}
          onClick={() => {
            handleToggleSettings('counter24h.fail');
          }}
        />
        <ColumnOption
          text={t('counter24h.reject')}
          isChecked={checkedSettings['counter24h.reject']}
          onClick={() => {
            handleToggleSettings('counter24h.reject');
          }}
        />
        <Divider sx={{ my: 1, mx: { xs: -4, md: 0 } }} />
        <Typography variant="h5" sx={{ px: 3, pt: 2, pb: 1 }}>
          {t('currentFwds')}
        </Typography>
        <ColumnOption
          text={t('pendingHtlcCount')}
          isChecked={checkedSettings.pendingHtlcCount}
          onClick={() => {
            handleToggleSettings('pendingHtlcCount');
          }}
        />
        <ColumnOption
          text={t('queueLen')}
          isChecked={checkedSettings.queueLen}
          onClick={() => {
            handleToggleSettings('queueLen');
          }}
        />
        <Divider sx={{ my: 1, mx: { xs: -4, md: 0 } }} />
        <Typography variant="h5" sx={{ px: 3, pt: 2, pb: 1 }}>
          {t('limit.title')}
        </Typography>
        <ColumnOption
          text={t('limit.maxHourlyRate')}
          isChecked={checkedSettings['limit.maxHourlyRate']}
          onClick={() => {
            handleToggleSettings('limit.maxHourlyRate');
          }}
        />
        <ColumnOption
          text={t('limit.maxPending')}
          isChecked={checkedSettings['limit.maxPending']}
          onClick={() => {
            handleToggleSettings('limit.maxPending');
          }}
        />
        <ColumnOption
          text={t('limit.mode')}
          isChecked={checkedSettings['limit.mode']}
          onClick={() => {
            handleToggleSettings('limit.mode');
          }}
        />
      </List>
    </ClickAwayListener>
  );

  return (
    <Box id="main">
      {isTablet ? (
        <Box sx={{ position: 'relative' }}>
          <SecondaryButton
            onClick={() => setShowSettings(!showSettings)}
            sx={{
              height: '40px',
              ...(showSettings && {
                boxShadow: 'inset 0px 2px 6px rgba(0, 0, 0, 0.25)',
              }),
              '&:hover': {
                backgroundColor: 'grey.50',
              },
            }}
          >
            <Box sx={{ display: 'flex' }}>
              <Box sx={{ mr: 2, display: 'flex' }}>
                <Image
                  src="/icons/columns.svg"
                  alt="columns"
                  width={16}
                  height={16}
                />
              </Box>
              <Typography>{t('columns')}</Typography>
            </Box>
          </SecondaryButton>
          <Box
            sx={{
              display: { xs: 'none', md: 'block' },
              mt: 1,
              borderRadius: '8px',
              width: '208px',
              position: 'absolute',
              border: '1px solid #D3D4DB',
              zIndex: 100,
              backgroundColor: 'grey.100',
              filter:
                'filter: drop-shadow(0px 2px 2px rgba(0, 0, 0, 0.12)) drop-shadow(0px 4px 12px rgba(0, 0, 0, 0.16));',
              opacity: showSettings ? 1 : 0,
              visibility: showSettings ? 'visible' : 'hidden',
              transition: 'all 150ms ease-in-out',
            }}
          >
            <ColumnSettingOptions />
          </Box>
        </Box>
      ) : (
        <>
          <SecondaryButton
            onClick={() => setShowSettings(!showSettings)}
            sx={{
              ...(showSettings && {
                boxShadow: 'inset 0px 2px 6px rgba(0, 0, 0, 0.25)',
              }),
              '&:hover': {
                backgroundColor: 'grey.50',
              },
            }}
          >
            <Box sx={{ display: 'flex' }}>
              <Box sx={{ mr: 0, display: 'flex' }}>
                <Image
                  src="/icons/columns.svg"
                  alt="columns"
                  width={16}
                  height={16}
                />
              </Box>
            </Box>
          </SecondaryButton>
          <Drawer
            open={showSettings}
            anchor="bottom"
            sx={{
              zIndex: 101,
              display: { xs: 'initial', md: 'none' },
            }}
          >
            <ColumnSettingOptions />
          </Drawer>
        </>
      )}
    </Box>
  );
};

export default ColumnsSetting;
