import { useState } from 'react';
import {
  Box,
  Button,
  Typography,
  CircularProgress,
  Card,
  CardContent,
  CardActions,
  Dialog,
  DialogTitle,
  DialogContent,
  DialogActions,
  TextField,
  Alert,
} from '@mui/material';
import { useNavigate } from 'react-router-dom';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { listStores, setFlag } from '../services/api';

export const StoresPage = () => {
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [isCreateDialogOpen, setIsCreateDialogOpen] = useState(false);
  const [newStoreName, setNewStoreName] = useState('');

  const { data: stores, isLoading, error } = useQuery<string[], Error>({
    queryKey: ['stores'],
    queryFn: listStores,
  });

  const handleCreateStore = async () => {
    if (newStoreName) {
      try {
        // Create a default flag in the new store to initialize it
        await setFlag(newStoreName, '__default', { value: true });
        setIsCreateDialogOpen(false);
        setNewStoreName('');
        // Refresh stores list
        queryClient.invalidateQueries({ queryKey: ['stores'] });
      } catch (err) {
        console.error('Failed to create store:', err);
      }
    }
  };

  if (isLoading) {
    return (
      <Box sx={{ display: 'flex', justifyContent: 'center', mt: 4 }}>
        <CircularProgress />
      </Box>
    );
  }

  if (error) {
    return (
      <Box sx={{ mt: 4 }}>
        <Alert severity="error">
          Failed to load stores: {error.message}
        </Alert>
      </Box>
    );
  }

  return (
    <Box>
      <Box sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', mb: 4 }}>
        <Typography variant="h4">Feature Flag Stores</Typography>
        <Button
          variant="contained"
          color="primary"
          onClick={() => setIsCreateDialogOpen(true)}
        >
          Create Store
        </Button>
      </Box>

      {!stores || !Array.isArray(stores) || stores.length === 0 ? (
        <Typography variant="body1" color="text.secondary">
          No stores found. Create your first store to get started.
        </Typography>
      ) : (
        <Box sx={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(300px, 1fr))', gap: 2 }}>
          {stores.map((store) => (
            <Card key={store}>
              <CardContent>
                <Typography variant="h6">{store}</Typography>
              </CardContent>
              <CardActions>
                <Button size="small" onClick={() => navigate(`/stores/${store}`)}>
                  View Flags
                </Button>
              </CardActions>
            </Card>
          ))}
        </Box>
      )}

      {/* Create Store Dialog */}
      <Dialog open={isCreateDialogOpen} onClose={() => setIsCreateDialogOpen(false)}>
        <DialogTitle>Create New Store</DialogTitle>
        <DialogContent>
          <TextField
            autoFocus
            margin="dense"
            label="Store Name"
            fullWidth
            value={newStoreName}
            onChange={(e: React.ChangeEvent<HTMLInputElement>) => setNewStoreName(e.target.value)}
          />
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setIsCreateDialogOpen(false)}>Cancel</Button>
          <Button onClick={handleCreateStore} variant="contained" disabled={!newStoreName}>
            Create
          </Button>
        </DialogActions>
      </Dialog>
    </Box>
  );
}; 