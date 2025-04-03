import { useState } from 'react';
import {
  Dialog,
  DialogTitle,
  DialogContent,
  DialogActions,
  Button,
  TextField,
  Box,
} from '@mui/material';
import { FeatureFlag } from '../types/flag';

interface CreateFlagDialogProps {
  open: boolean;
  onClose: () => void;
  onCreate: (flag: Partial<FeatureFlag>) => void;
  store: string;
}

export const CreateFlagDialog = ({ open, onClose, onCreate, store }: CreateFlagDialogProps) => {
  const [newFlag, setNewFlag] = useState<Partial<FeatureFlag>>({
    key: '',
    value: '',
    expression: '',
    store,
  });

  const handleCreate = () => {
    onCreate(newFlag);
    onClose();
    setNewFlag({
      key: '',
      value: '',
      expression: '',
      store,
    });
  };

  return (
    <Dialog open={open} onClose={onClose}>
      <DialogTitle>Create New Feature Flag</DialogTitle>
      <DialogContent>
        <Box sx={{ display: 'flex', flexDirection: 'column', gap: 2, pt: 2 }}>
          <TextField
            label="Key"
            value={newFlag.key}
            onChange={(e) => setNewFlag({ ...newFlag, key: e.target.value })}
            fullWidth
            required
          />
          <TextField
            label="Value"
            value={newFlag.value}
            onChange={(e) => setNewFlag({ ...newFlag, value: e.target.value })}
            fullWidth
            required
          />
          <TextField
            label="CEL Expression"
            value={newFlag.expression}
            onChange={(e) => setNewFlag({ ...newFlag, expression: e.target.value })}
            fullWidth
            multiline
            rows={2}
            placeholder="e.g., context.region == 'us-west' && context.environment == 'prod'"
          />
        </Box>
      </DialogContent>
      <DialogActions>
        <Button onClick={onClose}>Cancel</Button>
        <Button onClick={handleCreate} variant="contained" disabled={!newFlag.key || !newFlag.value}>
          Create
        </Button>
      </DialogActions>
    </Dialog>
  );
}; 