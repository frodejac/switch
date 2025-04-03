import { useState } from 'react';
import { Box, Button, Typography, CircularProgress } from '@mui/material';
import { useParams, useNavigate } from 'react-router-dom';
import { FlagCard } from '../components/FlagCard';
import { CreateFlagDialog } from '../components/CreateFlagDialog';
import { useFlags } from '../hooks/useFlags';
import { FeatureFlag } from '../types/flag';
import AddIcon from '@mui/icons-material/Add';

export const FlagsPage = () => {
  const { store } = useParams<{ store: string }>();
  const navigate = useNavigate();
  const [isCreateDialogOpen, setIsCreateDialogOpen] = useState(false);
  const { flags, isLoading, updateFlag, removeFlag } = useFlags(store || 'default');

  const handleCreateFlag = (newFlag: Partial<FeatureFlag>) => {
    if (newFlag.key) {
      updateFlag({ key: newFlag.key, flag: newFlag });
    }
  };

  if (isLoading) {
    return (
      <Box sx={{ display: 'flex', justifyContent: 'center', mt: 4 }}>
        <CircularProgress />
      </Box>
    );
  }

  return (
    <Box>
      <Box sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', mb: 2 }}>
        <Box>
          <Button onClick={() => navigate('/')} sx={{ mr: 2 }}>
            ← Back to Stores
          </Button>
          <Typography variant="h4" component="span" sx={{ fontSize: '1.5rem' }}>
            {store} Flags
          </Typography>
        </Box>
        <Button
          variant="contained"
          color="primary"
          onClick={() => setIsCreateDialogOpen(true)}
          startIcon={<AddIcon />}
        >
          New Flag
        </Button>
      </Box>

      {flags?.length === 0 ? (
        <Typography variant="body1" color="text.secondary" sx={{ mt: 2 }}>
          No feature flags found. Create your first flag to get started.
        </Typography>
      ) : (
        flags?.map((flag) => (
          <FlagCard
            key={flag.key}
            flag={flag}
            onUpdate={(key, updatedFlag) => updateFlag({ key, flag: updatedFlag })}
            onDelete={removeFlag}
          />
        ))
      )}

      <CreateFlagDialog
        open={isCreateDialogOpen}
        onClose={() => setIsCreateDialogOpen(false)}
        onCreate={handleCreateFlag}
        store={store || 'default'}
      />
    </Box>
  );
}; 