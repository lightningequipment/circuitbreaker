import { useQuery } from '@tanstack/react-query';
import { AxiosError } from 'axios';

import { getInfo } from 'services/circuitbreaker';

const useInfo = () => {
  const query = useQuery<Info, AxiosError<APIError>>({
    queryKey: ['info'],
    queryFn: getInfo,
    staleTime: 0,
    retry: false,
  });

  return { info: { ...query.data }, ...query };
};

export default useInfo;
