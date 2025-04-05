import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { FeatureFlag, FlagContext } from '../types/flag';
import { getFlag, setFlag, listFlags, deleteFlag } from '../services/api';

export const useFlags = (store: string) => {
  const queryClient = useQueryClient();

  const { data: flags, isLoading } = useQuery({
    queryKey: ['flags', store],
    queryFn: () => listFlags(store),
  });

  const { mutate: updateFlag } = useMutation({
    mutationFn: ({ key, flag }: { key: string; flag: Partial<FeatureFlag> }) =>
      setFlag(store, key, flag),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['flags', store] });
    },
  });

  const { mutate: removeFlag } = useMutation({
    mutationFn: (key: string) => deleteFlag(store, key),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['flags', store] });
    },
  });

  const getFlagValue = async (key: string, context?: FlagContext) => {
    return getFlag(store, key, context);
  };

  return {
    flags,
    isLoading,
    updateFlag,
    removeFlag,
    getFlagValue,
  };
}; 