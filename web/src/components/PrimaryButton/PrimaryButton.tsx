import { Button, ButtonProps } from '@mui/material';

const PrimaryButton = ({ sx, ...rest }: ButtonProps) => (
  <Button
    sx={{
      backgroundColor: 'primary.main',
      color: 'grey.50',
      height: '39px',
      '&:hover': {
        backgroundColor: '#387AFF',
      },
      ...sx,
    }}
    {...rest}
  />
);

export default PrimaryButton;
