import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { useMutation } from '@tanstack/react-query';

import { EditLimitsForm, Modal, PrimaryButton } from 'components';
import { useLimits } from 'hooks';
import { clearLimits, updateLimits } from 'services/circuitbreaker';
import { isEqual } from 'lodash';
import {
  Accordion,
  AccordionDetails,
  AccordionSummary,
  Box,
  Divider,
  Typography,
} from '@mui/material';
import Image from 'next/image';

interface EditSelectedNodesProps {
  selected: string[];
}

const EditSelectedNodes = ({ selected }: EditSelectedNodesProps) => {
  const { t } = useTranslation('common', { keyPrefix: 'node-table' });

  const [isModalOpen, setIsModalOpen] = useState(false);

  const { data, refetch } = useLimits();

  const handleModalClose = () => setIsModalOpen(false);
  const isButtonDisabled = !selected.length;

  const updateSelected = async (newLimit: Limit) => {
    const { defaultLimit } = data!;
    const isNewLimitDefault = isEqual(newLimit, defaultLimit);
    if (isNewLimitDefault) {
      await clearLimits({ nodes: selected });
    } else {
      const newLimits = selected.reduce(
        (obj, nodeId) => ({
          ...obj,
          [nodeId]: newLimit,
        }),
        {}
      );
      await updateLimits({
        limits: newLimits,
      });
    }
  };

  const { mutate } = useMutation({
    mutationFn: updateSelected,
    onSuccess: () => {
      refetch();
      handleModalClose();
    },
  });

  const handleSubmit = (newLimit: Limit) => {
    mutate(newLimit);
  };

  const getSelectedNode = () => {
    if (selected.length === 1) {
      const { limits } = data!;
      const nodeLimit = limits.find((n) => n.node === selected[0]);
      return nodeLimit?.limit;
    }
    return undefined;
  };

  return (
    <>
      <PrimaryButton
        onClick={() => setIsModalOpen(true)}
        disabled={isButtonDisabled}
      >
        <Box sx={{ display: 'flex' }}>
          <Box sx={{ mr: { xs: 0, md: 2 }, display: 'flex' }}>
            <Image
              src={
                isButtonDisabled
                  ? '/icons/edit-disabled.svg'
                  : '/icons/edit.svg'
              }
              alt="columns"
              width={16}
              height={16}
            />
          </Box>
          <Typography
            sx={{
              display: { xs: 'none', md: 'block' },
              color: isButtonDisabled ? '#828CC5' : 'grey.50',
            }}
          >
            {t('edit-selected')}
          </Typography>
        </Box>
      </PrimaryButton>
      <Modal
        title={t('edit-selected-nodes') || undefined}
        open={isModalOpen}
        onClose={handleModalClose}
      >
        <EditLimitsForm
          initialValues={getSelectedNode()}
          onCancel={handleModalClose}
          onSubmit={handleSubmit}
          isNodeEdit
        />
        <Box sx={{ mt: 6 }}>
          <Divider sx={{ mx: -6, mb: 6 }} />
          <Accordion>
            <AccordionSummary
              expandIcon={
                <Image
                  src="/icons/caret-down.svg"
                  alt="expand"
                  height="12"
                  width="12"
                />
              }
              aria-controls="panel1a-content"
              id="panel1a-header"
            >
              <Typography>{t('selected-peers')}</Typography>
            </AccordionSummary>
            <AccordionDetails>
              <Typography sx={{ color: '#5C6484' }}>
                {selected.map(
                  (nodeId, index) =>
                    `${
                      data!.limits.find(({ node }) => node === nodeId)?.alias
                    }${index < selected.length - 1 ? ', ' : ''}`
                )}
              </Typography>
            </AccordionDetails>
          </Accordion>
        </Box>
      </Modal>
    </>
  );
};

export default EditSelectedNodes;
