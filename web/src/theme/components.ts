import { Theme } from '@mui/material';

const components: Theme['components'] = {
  MuiButton: {
    styleOverrides: {
      root: ({ theme }) =>
        theme.unstable_sx({
          textTransform: 'none',
          whiteSpace: 'nowrap',
          borderRadius: '8px',
          fontSize: '12px',
          px: 4,
          py: 3,
          '&:disabled': {
            backgroundColor: '#E4EAFF',
            color: '#828CC5',
            border: 'none',
          },
        }),
    },
  },
  MuiTable: {
    styleOverrides: {
      root: ({ theme }) => theme.unstable_sx({}),
    },
  },
  MuiTableHead: {
    styleOverrides: {
      root: ({ theme }) =>
        theme.unstable_sx({
          '.MuiTableRow-root': {
            ':first-of-type': {
              '.MuiTableCell-root': {
                borderTopStyle: 'solid',
                ':first-of-type': {
                  borderTopLeftRadius: '12px',
                },
                ':last-of-type': {
                  borderTopRightRadius: '12px',
                },
              },
            },
            ':last-of-type': {
              '.MuiTableCell-root': {
                ':last-of-type': {
                  borderBottomRightRadius: '12px',
                },
              },
            },
            '.MuiTableCell-root': {
              ':first-of-type': {
                borderLeftStyle: 'solid',
              },
              borderRightStyle: 'solid',
              borderBottomStyle: 'solid',
              borderWidth: '1px',
              backgroundColor: 'grey.100',
            },
          },
        }),
    },
  },
  MuiTableBody: {
    styleOverrides: {
      root: ({ theme }) =>
        theme.unstable_sx({
          '.MuiTableRow-root:nth-of-type(even):not(.Mui-selected)': {
            backgroundColor: 'grey.200',
          },
        }),
    },
  },
  MuiTableContainer: {
    styleOverrides: {
      root: ({ theme }) =>
        theme.unstable_sx({
          borderTopLeftRadius: '12px',
          borderTopRightRadius: '12px',
        }),
    },
  },
  MuiTableRow: {
    styleOverrides: {
      root: ({ theme }) =>
        theme.unstable_sx({
          '&.Mui-selected': {
            backgroundColor: '#EBEFFF',
          },
          '&:hover': {
            backgroundColor: '#E5E6EE !important',
          },
        }),
    },
  },
  MuiTableCell: {
    styleOverrides: {
      root: ({ theme }) =>
        theme.unstable_sx({
          whiteSpace: 'nowrap',
          borderColor: 'grey.300',
          p: 3,
          fontSize: '12px',
          color: 'grey.900',
          lineHeight: 1.25,
        }),
    },
  },
  MuiModal: {
    styleOverrides: {
      root: ({ theme }) =>
        theme.unstable_sx({
          '.MuiDialog-paper': {
            backgroundColor: 'grey.100',
            p: 6,
            borderRadius: '8px',
            width: '325px',
          },
          '.MuiDrawer-paper': {
            borderTopLeftRadius: '8px',
            borderTopRightRadius: '8px',
            backgroundColor: 'grey.100',
            p: 4,
            pb: 8,
          },
        }),
    },
  },
  MuiInputLabel: {
    styleOverrides: {
      root: ({ theme }) =>
        theme.unstable_sx({
          fontWeight: 500,
          color: '#5C6484',
          mb: '6px',
          '&.Mui-disabled': {
            color: '#9DA3C5',
          },
        }),
    },
  },
  MuiInputBase: {
    styleOverrides: {
      root: ({ theme }) =>
        theme.unstable_sx({
          '.MuiInputBase-input': {
            fontSize: 16,
            py: 0,
            lineHeight: '23px',
            '::placeholder': {
              color: '#6E6F78 !important',
              opacity: 1,
            },
            '&.MuiInputBase-inputAdornedStart': {
              pl: 2,
            },
          },
          height: '39px',
          px: 4,
          py: 2,
          borderRadius: '8px',
          backgroundColor: '#FFFFFF',
          border: '1px solid #D3D4DB',
          '&:not(.Mui-disabled)': {
            '&:hover, &.Mui-focused': {
              borderColor: '#C5C7D6',
              boxShadow: '0 0 0 2px #E8ECFF',
            },
          },
          '&.Mui-disabled': {
            backgroundColor: '#E4E6EE',
            borderColor: '#E4E6EE',
            '.MuiInputBase-input': {
              color: '#9DA3C5',
            },
          },
        }),
    },
  },
  MuiCheckbox: {
    styleOverrides: {
      root: ({ theme }) =>
        theme.unstable_sx({
          backgroundColor: 'grey.50',
          borderRadius: '8px !important',
          padding: '0 !important',
        }),
    },
  },
  MuiDivider: {
    styleOverrides: {
      root: ({ theme }) =>
        theme.unstable_sx({
          borderColor: '#D3D4DB',
        }),
    },
  },
  MuiListItemButton: {
    styleOverrides: {
      root: ({ theme }) =>
        theme.unstable_sx({
          py: 1,
        }),
    },
  },
  MuiListItemText: {
    styleOverrides: {
      root: ({ theme }) =>
        theme.unstable_sx({
          color: '#474A59',
        }),
    },
  },
  // ACCORDION STYLES
  MuiAccordion: {
    styleOverrides: {
      root: ({ theme }) =>
        theme.unstable_sx({
          p: 0,
          backgroundColor: 'transparent',
          boxShadow: 'none',
        }),
    },
  },
  MuiAccordionSummary: {
    styleOverrides: {
      root: ({ theme }) =>
        theme.unstable_sx({
          p: 0,
          justifyContent: 'flex-start',
          minHeight: '0 !important',
          '.MuiAccordionSummary-content, .MuiAccordionSummary-content.Mui-expanded':
            {
              m: 0,
              flexGrow: 0,
              mr: 1,
            },
        }),
    },
  },
  MuiAccordionDetails: {
    styleOverrides: {
      root: ({ theme }) =>
        theme.unstable_sx({
          p: 0,
          pt: 2,
        }),
    },
  },
  // SELECT STYLES
  MuiPopover: {
    styleOverrides: {
      root: ({ theme }) =>
        theme.unstable_sx({
          '.MuiPopover-paper': {
            background: '#F6F6FA',
            borderRadius: '8px',
          },
          '.MuiMenu-list': {
            p: 0,
            '.MuiMenuItem-root': {
              px: 4,
              py: 3,
              fontSize: '16px',
              '&:not(:last-of-type)': {
                borderBottom: '1px solid #D3D4DB',
              },
            },
          },
        }),
    },
  },
  MuiTablePagination: {
    styleOverrides: {
      root: ({ theme }) =>
        theme.unstable_sx({
          '.MuiToolbar-root': {
            pl: 0,
            minHeight: '0',
            mt: 6,
          },
          '.MuiTablePagination-selectLabel': {
            fontSize: '12px',
            m: 0,
            mr: 2,
          },
          '.MuiTablePagination-select': {
            fontSize: '12px',
          },
          '.MuiTablePagination-displayedRows': {
            minWidth: '20px',
            fontSize: '12px',
            order: 4,
            m: 0,
          },
          '.MuiTablePagination-actions': {
            ml: 0,
            mr: 2,
          },
          '.MuiInputBase-root': {
            height: '26px',
            px: 2,
            minWidth: '70px',
            m: 0,
            mr: 2,
          },
        }),
    },
  },
  MuiTooltip: {
    styleOverrides: {
      popper: ({ theme }) =>
        theme.unstable_sx({
          '.MuiTooltip-tooltip': {
            backgroundColor: '#F6F6FA',
            p: 4,
            borderRadius: '8px',
            boxShadow:
              '0px 2px 2px rgba(0, 0, 0, 0.12), 0px 4px 12px rgba(0, 0, 0, 0.16)',
          },
        }),
    },
  },
};

export default components;
