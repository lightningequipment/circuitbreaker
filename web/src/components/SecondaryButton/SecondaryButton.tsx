import { Button, ButtonProps } from '@mui/material';

const SecondaryButton = ({ sx, ...rest }: ButtonProps) => (
  <Button
    sx={{
      backgroundColor: 'grey.100',
      color: '#5C6484',
      border: '1px solid',
      borderColor: '#D3D4DB',
      height: '39px',
      '&:hover, &:focus': {
        borderColor: '#C5C7D6',
      },
      '&:active': {
        backgroundColor: '#F2F2F7',
      },
      ...sx,
    }}
    {...rest}
  />
);

export default SecondaryButton;
