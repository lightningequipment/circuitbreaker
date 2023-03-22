import React from 'react';
import { useTranslation } from 'react-i18next';
import { IconButton, Typography, Box } from '@mui/material';

import Image from 'next/image';

interface TableSortLabelProps {
  column: ColumnId;
  handleRequestSort: (column: ColumnId) => void;
  orderBy: ColumnId;
  order: Order;
}

const TableSortLabel = ({
  column,
  order,
  orderBy,
  handleRequestSort,
}: TableSortLabelProps) => {
  const { t } = useTranslation('common', { keyPrefix: 'node-table' });

  const isActive = orderBy === column;

  return (
    <Box style={{ display: 'flex', alignItems: 'center' }}>
      <Typography>{t(column)}</Typography>
      <IconButton
        onClick={(e) => {
          e.preventDefault();
          handleRequestSort(column);
        }}
        sx={{
          p: 1,
        }}
      >
        {isActive && order === 'asc' && (
          <Image src="/icons/sort-asc.svg" width={12} height={12} alt="icon" />
        )}
        {isActive && order === 'desc' && (
          <Image src="/icons/sort-desc.svg" width={12} height={12} alt="icon" />
        )}
        {!isActive && (
          <Image src="/icons/sort.svg" width={12} height={12} alt="icon" />
        )}
      </IconButton>
    </Box>
  );
};

export default TableSortLabel;
