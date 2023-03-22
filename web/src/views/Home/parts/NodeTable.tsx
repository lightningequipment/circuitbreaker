import React, { useState } from 'react';
import get from 'lodash/get';
import { useTranslation } from 'react-i18next';
import { TFunction } from 'i18next';

import {
  Box,
  SxProps,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TablePagination,
  TableRow,
  Tooltip,
  Typography,
} from '@mui/material';

import { useLimits } from 'hooks';
import { Checkbox, NodeAlias } from 'components';
import { HEADER_HEIGHT_DESKTOP, HEADER_HEIGHT_MOBILE } from 'constant';
import { Mode } from 'enums';

import SearchBar from './SearchBar';
import ColumnsSetting from './ColumnsSetting';
import EditSelectedNodes from './EditSelectedNodes';
import TableSortIcon from './TableSortIcon';

interface Column {
  id: ColumnId;
  headerStyles?: SxProps;
  format?: (t: TFunction, nodeLimit: NodeLimit, value?: any) => React.ReactNode;
  style?: (value?: any) => SxProps;
}

interface NodeTableHeaderCellProps {
  columnGroups: ColumnId[][];
  order: Order;
  orderBy: ColumnId;
  handleRequestSort: (property: ColumnId) => void;
}

const defaultCellFormat = (t: TFunction, nodeLimit: NodeLimit, value: string) =>
  value === '0' ? '-' : value;
const defaultCellStyle = (value: string) =>
  value === '0' ? { color: '#CFD0DB' } : {};
const limitStyle = (value?: string) => (value ? {} : { color: '#6E6F78' });

const columns: Column[] = [
  {
    id: 'alias',
    format: (t: TFunction, nodeLimit: NodeLimit, value?: string) => (
      <Tooltip
        enterTouchDelay={0}
        placement="bottom-end"
        title={
          <Box>
            {value && (
              <Typography sx={{ color: '#5C6484', mb: 1 }}>{value}</Typography>
            )}
            <Typography sx={{ mb: 3 }}>{nodeLimit.node}</Typography>
          </Box>
        }
      >
        <Box sx={{ cursor: 'pointer' }}>
          <NodeAlias nodeLimit={nodeLimit} alias={value} />
        </Box>
      </Tooltip>
    ),
    style: defaultCellStyle,
  },
  {
    id: 'counter1h.success',
    format: defaultCellFormat,
    style: defaultCellStyle,
  },
  { id: 'counter1h.fail', format: defaultCellFormat, style: defaultCellStyle },
  {
    id: 'counter1h.reject',
    format: defaultCellFormat,
    style: defaultCellStyle,
  },
  {
    id: 'counter24h.success',
    format: defaultCellFormat,
    style: defaultCellStyle,
  },
  { id: 'counter24h.fail', format: defaultCellFormat, style: defaultCellStyle },
  {
    id: 'counter24h.reject',
    format: defaultCellFormat,
    style: defaultCellStyle,
  },
  {
    id: 'pendingHtlcCount',
    format: defaultCellFormat,
    style: defaultCellStyle,
  },
  { id: 'queueLen', format: defaultCellFormat, style: defaultCellStyle },
  {
    id: 'limit.maxHourlyRate',
    format: (t: TFunction, nodeLimit: NodeLimit, value?: string) => {
      if (!nodeLimit.limit) return t('default');
      if (nodeLimit.limit.mode === Mode.Block) return '-';
      if (value === '0') return '∞';
      return value!;
    },
    style: limitStyle,
  },
  {
    id: 'limit.maxPending',
    format: (t: TFunction, nodeLimit: NodeLimit, value?: string) => {
      if (!nodeLimit.limit) return t('default');
      if (nodeLimit.limit.mode === Mode.Block) return '-';
      if (value === '0') return '∞';
      return value!;
    },
    style: limitStyle,
  },
  {
    id: 'limit.mode',
    format: (t: TFunction, nodeLimit: NodeLimit, value?: string) => {
      if (!nodeLimit.limit) return t('default');
      if (value === Mode.Block) return t(value!);
      if (
        nodeLimit.limit.maxHourlyRate === '0' &&
        nodeLimit.limit.maxPending === '0'
      )
        return '-';
      return t(value!);
    },
    style: limitStyle,
  },
];

function sortObjectsByKey(
  arr: NodeLimit[],
  key: ColumnId,
  direction: Order
): NodeLimit[] {
  return arr.sort((a: NodeLimit, b: NodeLimit) => {
    const aValue = get(a, key, 'Default'); // Default is used to sort limits
    const bValue = get(b, key, 'Default');

    const aIsNumber = !Number.isNaN(parseFloat(aValue));
    const bIsNumber = !Number.isNaN(parseFloat(bValue));

    let comparisonResult: number;

    if (aIsNumber && bIsNumber) {
      comparisonResult = parseFloat(aValue) - parseFloat(bValue);
    } else if (aIsNumber) {
      comparisonResult = -1;
    } else if (bIsNumber) {
      comparisonResult = 1;
    } else {
      comparisonResult = aValue.localeCompare(bValue);
    }

    return direction === 'asc' ? comparisonResult : -comparisonResult;
  });
}

const NodeTableHeaderRow = ({
  columnGroups,
  orderBy,
  order,
  handleRequestSort,
}: NodeTableHeaderCellProps) => {
  const numColumns = columnGroups.reduce((acc, curr) => acc + curr.length, 0);

  if (numColumns === 0) return null;

  return (
    <TableRow>
      {columnGroups.map((columnGroup, groupIndex) => (
        // eslint-disable-next-line
        <React.Fragment key={groupIndex}>
          {columnGroup.map((column, columnIndex) => (
            <TableCell
              key={column}
              sx={{
                top: 41,
                ...(columnIndex === 0 &&
                  columnIndex !== columnGroup.length - 1 && {
                    borderLeftColor: 'grey.100',
                    borderRightColor: 'grey.100',
                  }),
                ...(columnIndex !== columnGroup.length - 1 && {
                  borderRightColor: 'grey.100',
                }),
              }}
            >
              <TableSortIcon
                column={column}
                order={order}
                orderBy={orderBy}
                handleRequestSort={handleRequestSort}
              />
            </TableCell>
          ))}
        </React.Fragment>
      ))}
    </TableRow>
  );
};

const NodeTable = () => {
  const { t } = useTranslation('common', { keyPrefix: 'node-table' });

  const { data } = useLimits();

  const [page, setPage] = useState(0);
  const [rowsPerPage, setRowsPerPage] = useState(25);
  const [selected, setSelected] = useState<string[]>([]);
  const [searchValue, setSearchValue] = useState<string>('');
  const [order, setOrder] = React.useState<Order>('asc');
  const [orderBy, setOrderBy] = React.useState<ColumnId>('alias');
  const [checkedSettings, setCheckedSettings] = useState<{
    [K in ColumnId]: boolean;
  }>({
    alias: true,
    node: true,
    'counter1h.success': true,
    'counter1h.fail': true,
    'counter1h.reject': true,
    'counter24h.success': true,
    'counter24h.fail': true,
    'counter24h.reject': true,
    pendingHtlcCount: true,
    queueLen: true,
    'limit.maxPending': true,
    'limit.maxHourlyRate': true,
    'limit.mode': true,
  });

  if (!data) return null;

  const { limits: originalLimit } = data;

  const counter1hOptions: ColumnId[] = [
    'counter1h.success',
    'counter1h.fail',
    'counter1h.reject',
  ];

  const counter1hOptionsFiltered = counter1hOptions.filter(
    (columnId) => checkedSettings[columnId]
  );

  const counter24hOptions: ColumnId[] = [
    'counter24h.success',
    'counter24h.fail',
    'counter24h.reject',
  ];

  const counter24hOptionsFiltered = counter24hOptions.filter(
    (columnId) => checkedSettings[columnId]
  );

  const currentFwdsOptions: ColumnId[] = ['pendingHtlcCount', 'queueLen'];
  const currentFwdsOptionsFiltered = currentFwdsOptions.filter(
    (columnId) => checkedSettings[columnId]
  );

  const limitOptions: ColumnId[] = [
    'limit.maxHourlyRate',
    'limit.maxPending',
    'limit.mode',
  ];

  const limitOptionsFiltered = limitOptions.filter(
    (columnId) => checkedSettings[columnId]
  );

  const limits = searchValue
    ? originalLimit.filter((limit) =>
        limit.alias.toLowerCase().includes(searchValue.toLowerCase())
      )
    : originalLimit;

  const isSelected = (name: string) => selected.indexOf(name) !== -1;

  const handleToggleSettings = (value: ColumnId) => {
    setCheckedSettings((prev) => ({ ...prev, [value]: !prev[value] }));
  };

  const handleClick = (event: React.MouseEvent<unknown>, name: string) => {
    const selectedIndex = selected.indexOf(name);
    let newSelected: string[] = [];

    if (selectedIndex === -1) {
      newSelected = newSelected.concat(selected, name);
    } else if (selectedIndex === 0) {
      newSelected = newSelected.concat(selected.slice(1));
    } else if (selectedIndex === selected.length - 1) {
      newSelected = newSelected.concat(selected.slice(0, -1));
    } else if (selectedIndex > 0) {
      newSelected = newSelected.concat(
        selected.slice(0, selectedIndex),
        selected.slice(selectedIndex + 1)
      );
    }

    setSelected(newSelected);
  };

  const handleSelectAllClick = (event: React.ChangeEvent<HTMLInputElement>) => {
    if (event.target.checked) {
      const newSelected = limits.map((nodeLimit) => nodeLimit.node);
      setSelected(newSelected);
      return;
    }
    setSelected([]);
  };

  const handleChangePage = (event: unknown, newPage: number) => {
    setPage(newPage);
  };

  const handleChangeRowsPerPage = (
    event: React.ChangeEvent<HTMLInputElement>
  ) => {
    setRowsPerPage(+event.target.value);
    setPage(0);
  };

  const handleRequestSort = (property: ColumnId) => {
    const isAsc = orderBy === property && order === 'asc';
    setOrder(isAsc ? 'desc' : 'asc');
    setOrderBy(property);
  };

  return (
    <Box
      sx={{
        display: 'flex',
        flexDirection: 'column',
        height: {
          xs: `calc(100% - ${HEADER_HEIGHT_MOBILE})`,
          lg: `calc(100% - ${HEADER_HEIGHT_DESKTOP})`,
        },
        backgroundColor: 'grey.50',
        borderRadius: '12px',
        p: { xs: 4, md: 6 },
        pb: 8,
      }}
    >
      <Box sx={{ display: 'flex', mb: 5 }}>
        <Box
          sx={{
            display: 'flex',
            flex: 1,
            alignItems: 'center',
            mr: { xs: 2, md: 4 },
          }}
        >
          <Box sx={{ width: '100%', mr: { xs: 2, md: 4 } }}>
            <SearchBar
              placeholder={t('search-placeholder') as string}
              searchValue={searchValue}
              setSearchValue={(value: string) => {
                setPage(0);
                setSelected([]);
                setSearchValue(value);
              }}
            />
          </Box>
          <Box>
            <ColumnsSetting
              checkedSettings={checkedSettings}
              handleToggleSettings={handleToggleSettings}
            />
          </Box>
        </Box>
        <Box>
          <EditSelectedNodes selected={selected} />
        </Box>
      </Box>
      <TableContainer
        sx={{
          flex: 1,
          overflow: 'auto',
        }}
      >
        <Table stickyHeader aria-label="sticky table">
          <TableHead>
            <TableRow>
              <TableCell
                colSpan={1}
                rowSpan={2}
                sx={{
                  borderTopLeftRadius: '8px',
                  borderBottomLeftRadius: '8px',
                }}
              >
                <Checkbox
                  color="primary"
                  size="small"
                  indeterminate={
                    selected.length > 0 && selected.length < limits.length
                  }
                  checked={
                    limits.length > 0 && selected.length === limits.length
                  }
                  onChange={handleSelectAllClick}
                  inputProps={{
                    'aria-label': 'select all nodes',
                  }}
                />
              </TableCell>
              <TableCell colSpan={1} rowSpan={2} sx={{ minWidth: '150px' }}>
                <TableSortIcon
                  column="alias"
                  order={order}
                  orderBy={orderBy}
                  handleRequestSort={handleRequestSort}
                />
              </TableCell>
              {!!counter1hOptionsFiltered.length && (
                <TableCell
                  colSpan={counter1hOptionsFiltered.length}
                  sx={{ color: 'grey.800' }}
                >
                  {t('counter1h.title')}
                </TableCell>
              )}
              {!!counter24hOptionsFiltered.length && (
                <TableCell
                  colSpan={counter24hOptionsFiltered.length}
                  sx={{ color: 'grey.800' }}
                >
                  {t('counter24h.title')}
                </TableCell>
              )}
              {!!currentFwdsOptionsFiltered.length && (
                <TableCell
                  colSpan={currentFwdsOptionsFiltered.length}
                  sx={{ color: 'grey.800' }}
                >
                  {t('currentFwds')}
                </TableCell>
              )}
              {!!limitOptionsFiltered.length && (
                <TableCell
                  colSpan={limitOptionsFiltered.length}
                  sx={{ color: 'grey.800' }}
                >
                  {t('limit.title')}
                </TableCell>
              )}
            </TableRow>
            <NodeTableHeaderRow
              orderBy={orderBy}
              order={order}
              handleRequestSort={handleRequestSort}
              columnGroups={[
                counter1hOptionsFiltered,
                counter24hOptionsFiltered,
                currentFwdsOptionsFiltered,
                limitOptionsFiltered,
              ]}
            />
          </TableHead>
          <TableBody>
            {sortObjectsByKey(limits, orderBy, order)
              .slice(page * rowsPerPage, page * rowsPerPage + rowsPerPage)
              .map((nodeLimit) => {
                const isItemSelected = isSelected(nodeLimit.node);
                return (
                  <TableRow
                    key={nodeLimit.node}
                    onClick={(event) => handleClick(event, nodeLimit.node)}
                    hover
                    role="checkbox"
                    selected={isItemSelected}
                    aria-checked={isItemSelected}
                    tabIndex={-1}
                  >
                    <>
                      <TableCell padding="checkbox">
                        <Checkbox
                          size="small"
                          color="primary"
                          checked={isItemSelected}
                          inputProps={{
                            'aria-labelledby': nodeLimit.node,
                          }}
                        />
                      </TableCell>
                      {columns.map((column) => {
                        if (checkedSettings[column.id]) {
                          const value = get(nodeLimit, column.id);
                          return (
                            <TableCell
                              key={column.id}
                              sx={{ ...(column.style && column.style(value)) }}
                            >
                              {column.format
                                ? column.format(t, nodeLimit, value)
                                : value}
                            </TableCell>
                          );
                        }
                        return null;
                      })}
                    </>
                  </TableRow>
                );
              })}
          </TableBody>
        </Table>
      </TableContainer>
      <TablePagination
        rowsPerPageOptions={[10, 25, 100]}
        component="div"
        count={limits.length}
        rowsPerPage={rowsPerPage}
        page={page}
        onPageChange={handleChangePage}
        onRowsPerPageChange={handleChangeRowsPerPage}
        sx={{
          '& .MuiTablePagination-toolbar': {
            width: 'fit-content',
          },
        }}
      />
    </Box>
  );
};

export default NodeTable;
