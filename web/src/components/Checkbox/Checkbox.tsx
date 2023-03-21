import * as React from 'react';
import { styled } from '@mui/material/styles';
import MuiCheckbox, {
  CheckboxProps as MuiCheckboxProps,
} from '@mui/material/Checkbox';
import { get } from 'lodash';

interface StyleProps {
  variant: 'primary' | 'secondary';
}

const BpIcon = styled('span')<StyleProps>(({ theme }) => ({
  borderRadius: 4,
  width: 12,
  height: 12,
  border: `1px solid`,
  borderColor: `#C5C7D6`,
  backgroundColor: theme.palette.grey['50'],
  position: 'relative',
  '.Mui-focusVisible &': {
    boxShadow: '0 0 0 2px #E8ECFF',
  },
  'input:hover ~ &': {
    boxShadow: '0 0 0 2px #E8ECFF',
  },
  'input:disabled ~ &': {
    boxShadow: 'none',
    background: '#E4E6EE',
    borderColor: '#E8ECFF',
  },
}));

const variants = {
  primary: {
    backgroundColor: 'primary.main',
    iconName: 'check',
  },
  secondary: {
    backgroundColor: 'grey.50',
    iconName: 'check-primary',
  },
};
const BpCheckedIcon = styled(BpIcon)(({ theme, variant }) => ({
  backgroundColor: get(theme.palette, variants[variant].backgroundColor),
  border: '1px solid',
  borderColor: theme.palette.primary.main,
  '&:before': {
    display: 'block',
    position: 'absolute',
    top: 0,
    left: 0,
    bottom: 0,
    right: 0,
    backgroundImage: `url(/icons/${variants[variant].iconName}.svg)`,
    backgroundRepeat: 'no-repeat',
    backgroundSize: '8px 8px',
    backgroundPosition: 'center',
    content: '""',
  },
}));

const BpIndeterminateIcon = styled(BpCheckedIcon)({
  '&:before': {
    backgroundImage: 'url(/icons/minus.svg)',
  },
});

interface CheckboxProps extends MuiCheckboxProps, Partial<StyleProps> {}

// Inspired by blueprintjs
const Checkbox = ({ variant = 'primary', ...rest }: CheckboxProps) => (
  <MuiCheckbox
    disableRipple
    color="default"
    checkedIcon={<BpCheckedIcon variant={variant} />}
    icon={<BpIcon variant={variant} />}
    indeterminateIcon={<BpIndeterminateIcon variant={variant} />}
    inputProps={{ 'aria-label': 'Checkbox' }}
    sx={{
      '&:hover': { bgcolor: 'transparent' },
    }}
    {...rest}
  />
);

export default Checkbox;
