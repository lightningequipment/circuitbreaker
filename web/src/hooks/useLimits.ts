import { useQuery } from '@tanstack/react-query';
import { AxiosError } from 'axios';
import { enqueueSnackbar } from 'notistack';
import { useTranslation } from 'react-i18next';

import { getLimits } from 'services/circuitbreaker';

const useLimits = () => {
  const { t: tError } = useTranslation('common', { keyPrefix: 'errors' });

  const query = useQuery<Limits, AxiosError<APIError>>({
    queryKey: ['limits'],
    queryFn: getLimits,
    staleTime: 0,
    refetchInterval: 30000,
    retry: false,
    onError: () =>
      enqueueSnackbar(tError('error-fetching-limits'), {
        variant: 'error',
      }),
  });

  return query;
};

export default useLimits;
