import {useState} from 'react';
import {Box, Button, Dialog, DialogActions, DialogContent, DialogTitle, MenuItem, TextField,} from '@mui/material';
import {FeatureFlag, FeatureFlagType} from '../types/flag';

interface CreateFlagDialogProps {
  open: boolean;
  onClose: () => void;
  onCreate: (flag: Partial<FeatureFlag>) => void;
  store: string;
}

export const CreateFlagDialog = ({ open, onClose, onCreate, store }: CreateFlagDialogProps) => {
  const [newFlag, setNewFlag] = useState<Partial<FeatureFlag>>({
    key: '',
    value: false,
    type: FeatureFlagType.BOOLEAN,
    store,
  });

  const handleCreate = () => {
    onCreate(newFlag);
    onClose();
    setNewFlag({
      key: '',
      value: false,
      type: FeatureFlagType.BOOLEAN,
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
              label="Type"
              select
              value={newFlag.type}
              onChange={(e) => setNewFlag({ ...newFlag, type: e.target.value as FeatureFlagType })}
              fullWidth
              size="small"
          >
            {Object.values(FeatureFlagType).map((type) => (
                <MenuItem key={type} value={type}>
                  {type}
                </MenuItem>
            ))}
          </TextField>
          {(newFlag.type === FeatureFlagType.CEL || newFlag.type === FeatureFlagType.JSON) && (
              <TextField
                  label="CEL Expression"
                  value={newFlag.value || ''}
                  onChange={(e) => setNewFlag({ ...newFlag, value: e.target.value })}
                  fullWidth
                  multiline
                  rows={2}
                  size="small"
                  placeholder="e.g., context.region == 'us-west' && context.environment == 'prod'"
              />
          )}
          {(newFlag.type === FeatureFlagType.INT) && (
              <TextField
                  label="Value"
                  value={newFlag.value}
                  onChange={(e) => setNewFlag({ ...newFlag, value: parseInt(e.target.value) })}
                  fullWidth
                  required
              />
          )}
          {(newFlag.type === FeatureFlagType.FLOAT) && (
              <TextField
                  label="Value"
                  value={newFlag.value}
                  onChange={(e) => setNewFlag({ ...newFlag, value: parseFloat(e.target.value) })}
                  fullWidth
                  required
              />
          )}
          {(newFlag.type === FeatureFlagType.STRING) && (
              <TextField
                  label="Value"
                  value={newFlag.value}
                  onChange={(e) => setNewFlag({ ...newFlag, value: e.target.value })}
                  fullWidth
                  required
              />
          )}
          {(newFlag.type === FeatureFlagType.BOOLEAN) && (
              // Use radio buttons for boolean type
              <Box sx={{ display: 'flex', gap: 2 }}>
                  <TextField
                      label="Value"
                      select
                      value={newFlag.value ? 'true' : 'false'}
                      onChange={(e) => setNewFlag({ ...newFlag, value: (e.target.value === 'true') as boolean })}
                      fullWidth
                      required
                  >
                      <MenuItem value="true">True</MenuItem>
                      <MenuItem value="false">False</MenuItem>
                  </TextField>
              </Box>
          )}
        </Box>
      </DialogContent>
      <DialogActions>
        <Button onClick={onClose}>Cancel</Button>
        <Button onClick={handleCreate} variant="contained" disabled={newFlag.key === undefined || newFlag.key === '' || newFlag.value == undefined}>
          Create
        </Button>
      </DialogActions>
    </Dialog>
  );
}; 