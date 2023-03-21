import { useState } from 'react';
import Image from 'next/image';
import { useTranslation } from 'react-i18next';
import { useMutation } from '@tanstack/react-query';
import { Box, Typography } from '@mui/material';

import { Mode } from 'enums';
import { useLimits } from 'hooks';
import { Modal, PrimaryButton, EditLimitsForm } from 'components';
import { updateDefaultLimit } from 'services/circuitbreaker';

interface ConfigProps {
  type: string;
  value: string;
}

const Config = ({ type, value }: ConfigProps) => {
  const { t } = useTranslation();

  const { data } = useLimits();

  const { defaultLimit } = data!;

  const getString = () => {
    if (type === 'mode') {
      if (value === Mode.Block) return t(`modes.${value}`);
      if (defaultLimit.maxHourlyRate === '0' && defaultLimit.maxPending === '0')
        return '-';
      return t(`modes.${value}`);
    }
    if (defaultLimit.mode === Mode.Block) return '-';
    if (value === '0') return '∞';
    return value;
  };

  return (
    <Box
      sx={{
        display: 'flex',
        alignItems: 'center',
        width: { xs: 'auto', md: 123 },
        mr: { xs: 4, md: 8 },
        '.config-icon': { display: { xs: 'none', md: 'block' } },
      }}
    >
      <Box className="config-icon" sx={{ mr: 3, img: { display: 'block' } }}>
        <Image src={`/icons/${type}.svg`} alt={type} width={24} height={24} />
      </Box>
      <Box>
        <Typography sx={{ color: 'grey.700', mb: 1 }}>{t(type)}</Typography>
        <Typography sx={{ color: 'grey.50', fontSize: '16px' }}>
          {getString()}
        </Typography>
      </Box>
    </Box>
  );
};

const DefaultLimits = () => {
  const { t } = useTranslation();
  const [isModalOpen, setIsModalOpen] = useState(false);

  const { data, refetch } = useLimits();

  const handleModalClose = () => setIsModalOpen(false);

  const { mutate } = useMutation({
    mutationFn: updateDefaultLimit,
    onSuccess: () => {
      refetch();
      handleModalClose();
    },
  });

  const handleSubmit = (newLimit: Limit) => {
    mutate({ limit: newLimit! });
  };

  return (
    <>
      <Box sx={{ display: 'flex', justifyContent: 'space-between' }}>
        <Box sx={{ display: 'flex' }}>
          <Config
            type="max-hourly-rate"
            value={data!.defaultLimit.maxHourlyRate}
          />
          <Config type="max-pending" value={data!.defaultLimit.maxPending} />
          <Config type="mode" value={data!.defaultLimit.mode} />
        </Box>
        <Box sx={{ display: 'flex', alignItems: 'center' }}>
          <PrimaryButton onClick={() => setIsModalOpen(true)}>
            <Box sx={{ display: 'flex', alignItems: 'center' }}>
              <Box
                sx={{
                  mr: 2,
                  display: 'flex',
                  img: {
                    width: { xs: '12px', md: '16px' },
                    height: { xs: '12px', md: '16px' },
                  },
                }}
              >
                <Image
                  src="/icons/edit.svg"
                  alt="columns"
                  width={12}
                  height={12}
                />
              </Box>
              <Typography
                component="span"
                sx={{ color: 'inherit', display: { md: 'none' } }}
              >
                {t('edit')}
              </Typography>
              <Typography
                component="span"
                sx={{ color: 'inherit', display: { xs: 'none', md: 'block' } }}
              >
                {t('edit-defaults')}
              </Typography>
            </Box>
          </PrimaryButton>
        </Box>
      </Box>
      <Modal
        title={t('default-limit-modal.title') || undefined}
        open={isModalOpen}
        onClose={handleModalClose}
      >
        <EditLimitsForm
          onCancel={handleModalClose}
          onSubmit={handleSubmit}
          initialValues={data!.defaultLimit}
        />
      </Modal>
    </>
  );
};

export default DefaultLimits;
