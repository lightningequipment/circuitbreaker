import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import {
  Box,
  Select,
  MenuItem,
  InputLabel,
  InputBase,
  Typography,
  Alert,
} from '@mui/material';

import { Mode } from 'enums';
import { Checkbox, PrimaryButton, SecondaryButton } from 'components';
import { useLimits } from 'hooks';
import Image from 'next/image';

interface EditLimitsFormProps {
  initialValues?: Limit;
  onCancel: () => void;
  onSubmit: (newLimit: Limit) => void;
  isNodeEdit?: boolean;
}

const EditLimitsForm = ({
  initialValues,
  onCancel,
  onSubmit,
  isNodeEdit,
}: EditLimitsFormProps) => {
  const { t } = useTranslation();

  const { data } = useLimits();

  const [maxHourlyRate, setMaxHourlyRate] = useState(
    initialValues?.maxHourlyRate || ''
  );
  const [maxPending, setMaxPending] = useState(initialValues?.maxPending || '');
  const [mode, setMode] = useState<Mode>(initialValues?.mode || Mode.Fail);

  const [isUseDefaultValues, setIsUseDefaultValues] = useState(false);

  const [isMaxHourlyRateUnlimited, setIsMaxHourlyRateUnlimited] = useState(
    initialValues?.maxHourlyRate === '0'
  );
  const [isMaxPendingUnlimited, setIsMaxPendingUnlimited] = useState(
    initialValues?.maxPending === '0'
  );

  const handleUseDefaultValuesChecked = (checked: boolean) => {
    setIsUseDefaultValues(checked);
    const { defaultLimit } = data!;
    if (checked) {
      setMaxHourlyRate(defaultLimit.maxHourlyRate);
      if (defaultLimit.maxHourlyRate === '0') setIsMaxHourlyRateUnlimited(true);
      setMaxPending(defaultLimit.maxPending);
      if (defaultLimit.maxPending === '0') setIsMaxPendingUnlimited(true);
      setMode(defaultLimit.mode);
    } else {
      setMaxHourlyRate('');
      setIsMaxHourlyRateUnlimited(false);
      setMaxPending('');
      setIsMaxPendingUnlimited(false);
      setMode(Mode.Fail);
    }
  };

  const handleSubmit: React.FormEventHandler = (e) => {
    e.preventDefault();
    onSubmit({ maxHourlyRate, maxPending, mode });
  };

  const isModeBlock = mode === Mode.Block;
  const isModeQueue = mode === Mode.Queue || mode === Mode.QueuePeerInitiated;

  const isUnlimited = isMaxHourlyRateUnlimited && isMaxPendingUnlimited;

  const isButtonDisabled =
    !maxHourlyRate ||
    !maxPending ||
    !mode ||
    (initialValues &&
      maxHourlyRate === initialValues.maxHourlyRate &&
      maxPending === initialValues.maxPending &&
      mode === initialValues.mode);

  const handleModeChange = (value: Mode) => {
    setMode(value as Mode);
    if (value === Mode.Block) {
      if (!maxHourlyRate) {
        setMaxHourlyRate(data!.defaultLimit.maxHourlyRate);
        if (data!.defaultLimit.maxHourlyRate === '0')
          setIsMaxHourlyRateUnlimited(true);
      }
      if (!maxPending) {
        setMaxPending(data!.defaultLimit.maxPending);
        if (data!.defaultLimit.maxPending === '0')
          setIsMaxPendingUnlimited(true);
      }
    }
  };

  const getMaskedLimit = (value: string) => {
    if (isModeBlock) return '0';
    return value === '0' ? '∞' : value;
  };

  const getSelectValue = () => {
    if (mode === Mode.Block) return mode;
    return isUnlimited ? 'allow-all' : mode;
  };

  return (
    <Box component="form" onSubmit={handleSubmit}>
      {isNodeEdit && (
        <InputLabel
          sx={{
            backgroundColor: '#E4E6EE',
            p: 3,
            color: '#5C6484',
            borderRadius: '8px',
            cursor: 'pointer',
            mb: 4,
            ...(isUseDefaultValues && {
              backgroundColor: 'primary.main',
              color: 'grey.50',
            }),
          }}
        >
          <Checkbox
            variant="secondary"
            value={isUseDefaultValues}
            onChange={(e) => handleUseDefaultValuesChecked(e.target.checked)}
            sx={{
              mr: 2,
              'input:hover ~ span': {
                boxShadow: 'none',
              },
            }}
          />
          {t('use-default-values')}
        </InputLabel>
      )}
      <Box sx={{ display: 'flex', gap: 4 }}>
        <Box sx={{ flex: 1, mb: 4 }}>
          <Box sx={{ mb: '6px' }}>
            <InputLabel>{t('max-hourly-rate')}</InputLabel>
            <InputBase
              fullWidth
              type={maxHourlyRate !== '0' ? 'number' : 'text'}
              value={getMaskedLimit(maxHourlyRate)}
              inputProps={{ min: '1' }}
              onChange={(e) => {
                const {
                  target: { value },
                } = e;
                if (value === '0') setIsMaxHourlyRateUnlimited(true);
                setMaxHourlyRate(value);
              }}
              disabled={
                isMaxHourlyRateUnlimited || isUseDefaultValues || isModeBlock
              }
            />
          </Box>
          <Box sx={{ display: 'flex', alignItems: 'center' }}>
            <InputLabel
              disabled={isUseDefaultValues || isModeBlock}
              sx={{ mb: 0 }}
            >
              <Checkbox
                checked={isMaxHourlyRateUnlimited}
                onChange={(e) => {
                  setIsMaxHourlyRateUnlimited(e.target.checked);
                  if (e.target.checked) setMaxHourlyRate('0');
                  else setMaxHourlyRate('');
                }}
                disabled={isUseDefaultValues || isModeBlock}
                sx={{ mr: '6px' }}
              />
              {t('unlimited')}
            </InputLabel>
          </Box>
        </Box>
        <Box sx={{ flex: 1 }}>
          <Box sx={{ mb: '6px' }}>
            <InputLabel>{t('max-pending')}</InputLabel>
            <InputBase
              fullWidth
              type={maxPending !== '0' ? 'number' : 'text'}
              value={getMaskedLimit(maxPending)}
              inputProps={{ min: '1' }}
              onChange={(e) => {
                const {
                  target: { value },
                } = e;
                setMaxPending(value);
                if (value === '0') setIsMaxPendingUnlimited(true);
              }}
              disabled={
                isMaxPendingUnlimited || isUseDefaultValues || isModeBlock
              }
            />
          </Box>
          <Box sx={{ display: 'flex', alignItems: 'center' }}>
            <InputLabel
              disabled={isUseDefaultValues || isModeBlock}
              sx={{ mb: 0 }}
            >
              <Checkbox
                checked={isMaxPendingUnlimited}
                onChange={(e) => {
                  setIsMaxPendingUnlimited(e.target.checked);
                  if (e.target.checked) setMaxPending('0');
                  else setMaxPending('');
                }}
                disabled={isUseDefaultValues || isModeBlock}
                sx={{ mr: '6px' }}
              />
              {t('unlimited')}
            </InputLabel>
          </Box>
        </Box>
      </Box>
      <Box sx={{ mb: 4 }}>
        <InputLabel>{t('mode')}</InputLabel>
        <Select
          labelId="demo-simple-select-label"
          id="demo-simple-select"
          input={<InputBase />}
          value={getSelectValue()}
          fullWidth
          disabled={(mode !== Mode.Block && isUnlimited) || isUseDefaultValues}
          onChange={(e) => {
            e.preventDefault();
            handleModeChange(e.target.value as Mode);
          }}
          sx={{ mb: 2 }}
        >
          {!isModeBlock && isUnlimited && (
            <MenuItem value="allow-all">-</MenuItem>
          )}
          {Object.entries(Mode).map(([key, value]) => (
            <MenuItem value={value} key={key}>
              {t(`modes.${value}`)}
            </MenuItem>
          ))}
        </Select>
        {isModeBlock && (
          <Box sx={{ display: 'flex', alignItems: 'center' }}>
            <Box sx={{ mr: '6px', img: { display: 'block' } }}>
              <Image src="/icons/info.svg" width="14" height="14" alt="info" />
            </Box>
            <Typography sx={{ fontSize: '10px', color: '#5C6484' }}>
              {t('blocked-description')}
            </Typography>
          </Box>
        )}
        {isModeQueue && (
          <Box>
            <Alert severity="warning">{t('queue-alert')}</Alert>
          </Box>
        )}
      </Box>
      <Box sx={{ display: 'flex', gap: 4 }}>
        <PrimaryButton fullWidth type="submit" disabled={isButtonDisabled}>
          {t('save')}
        </PrimaryButton>
        <SecondaryButton fullWidth type="button" onClick={onCancel}>
          {t('cancel')}
        </SecondaryButton>
      </Box>
    </Box>
  );
};

export default EditLimitsForm;
