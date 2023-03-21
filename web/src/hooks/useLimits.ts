import { useQuery } from '@tanstack/react-query';
import { AxiosError } from 'axios';

import { getLimits } from 'services/circuitbreaker';

const useLimits = () => {
  const query = useQuery<Limits, AxiosError<APIError>>({
    queryKey: ['limits'],
    queryFn: getLimits,
    staleTime: 0,
    refetchInterval: 30000,
    retry: false,
  });

  return query;
};

export default useLimits;
