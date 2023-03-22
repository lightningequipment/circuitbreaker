import { useQuery } from '@tanstack/react-query';
import { AxiosError } from 'axios';
import { useTranslation } from 'react-i18next';
import { enqueueSnackbar } from 'notistack';

import { getInfo } from 'services/circuitbreaker';

const useInfo = () => {
  const { t: tError } = useTranslation('common', { keyPrefix: 'errors' });

  const query = useQuery<Info, AxiosError<APIError>>({
    queryKey: ['info'],
    queryFn: getInfo,
    staleTime: 0,
    retry: false,
    onError: () =>
      enqueueSnackbar(tError('error-fetching-info'), {
        variant: 'error',
      }),
  });

  return { info: { ...query.data }, ...query };
};

export default useInfo;
