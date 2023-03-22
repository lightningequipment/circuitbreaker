import { Dialog, DialogTitle, DialogProps, Drawer } from '@mui/material';

const Modal = ({
  title,
  children,
  ...rest
}: React.PropsWithChildren<DialogProps>) => (
  <>
    <Dialog sx={{ display: { xs: 'none', md: 'initial' } }} {...rest}>
      <DialogTitle sx={{ p: 0, mb: 4 }}>{title}</DialogTitle>
      {children}
    </Dialog>
    <Drawer
      anchor="bottom"
      sx={{
        display: { xs: 'initial', md: 'none' },
      }}
      {...rest}
    >
      <DialogTitle sx={{ p: 0, mb: 4 }}>{title}</DialogTitle>
      {children}
    </Drawer>
  </>
);

export default Modal;
