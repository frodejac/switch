import { useState } from 'react';
import {
  Card,
  CardContent,
  CardActions,
  Typography,
  TextField,
  Button,
  Box,
  Chip,
  IconButton,
  Dialog,
  DialogTitle,
  DialogContent,
  DialogActions,
  Alert,
} from '@mui/material';
import DeleteOutlineIcon from '@mui/icons-material/DeleteOutline';
import EditOutlinedIcon from '@mui/icons-material/EditOutlined';
import PlayArrowIcon from '@mui/icons-material/PlayArrow';
import { FeatureFlag } from '../types/flag';
import { getFlag } from '../services/api';

interface FlagCardProps {
  flag: FeatureFlag;
  onUpdate: (key: string, flag: Partial<FeatureFlag>) => void;
  onDelete: (key: string) => void;
}

export const FlagCard = ({ flag, onUpdate, onDelete }: FlagCardProps) => {
  const [isEditing, setIsEditing] = useState(false);
  const [isDeleteDialogOpen, setIsDeleteDialogOpen] = useState(false);
  const [isTryItOutDialogOpen, setIsTryItOutDialogOpen] = useState(false);
  const [tryItOutResult, setTryItOutResult] = useState<any>(null);
  const [tryItOutError, setTryItOutError] = useState<string | null>(null);
  const [editedFlag, setEditedFlag] = useState<Partial<FeatureFlag>>({
    value: flag.value,
    expression: flag.expression,
  });

  const handleSave = () => {
    onUpdate(flag.key, editedFlag);
    setIsEditing(false);
  };

  const handleCancel = () => {
    setEditedFlag({
      value: flag.value,
      expression: flag.expression,
    });
    setIsEditing(false);
  };

  const handleDelete = () => {
    onDelete(flag.key);
    setIsDeleteDialogOpen(false);
  };

  const handleTryItOut = async () => {
    try {
      setTryItOutError(null);
      const result = await getFlag(flag.store, flag.key);
      setTryItOutResult(result);
      setIsTryItOutDialogOpen(true);
    } catch (error) {
      setTryItOutError(error instanceof Error ? error.message : 'An error occurred');
      setIsTryItOutDialogOpen(true);
    }
  };

  const formatValue = (value: any) => {
    if (typeof value === 'boolean') {
      return (
        <Chip
          label={value ? 'Enabled' : 'Disabled'}
          color={value ? 'success' : 'default'}
          size="small"
        />
      );
    }
    if (typeof value === 'number') {
      return <Typography variant="body1">{value}</Typography>;
    }
    if (typeof value === 'string') {
      return <Typography variant="body1">{value}</Typography>;
    }
    return <Typography variant="body1">{JSON.stringify(value)}</Typography>;
  };

  return (
    <>
      <Card sx={{ mb: 1, boxShadow: 1 }}>
        <CardContent sx={{ p: 1.5 }}>
          <Box sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start', mb: 1 }}>
            <Typography variant="h6" sx={{ fontWeight: 600, fontSize: '1rem' }}>
              {flag.key}
            </Typography>
            <Box>
              <Button 
                size="small" 
                variant="outlined" 
                onClick={handleTryItOut} 
                sx={{ mr: 1 }} 
                startIcon={<PlayArrowIcon />}
              >
                Try it out
              </Button>
              <Button size="small" variant="outlined" onClick={() => setIsEditing(true)} sx={{ mr: 1 }} startIcon={<EditOutlinedIcon />}>
                Edit
              </Button>
              <Button
                size="small"
                variant="outlined"
                color="inherit"
                onClick={() => setIsDeleteDialogOpen(true)}
                startIcon={<DeleteOutlineIcon />}
                sx={{
                  '&:hover': {
                    color: 'error.main',
                    borderColor: 'error.main',
                  }
                }}
              >
                Delete
              </Button>
            </Box>
          </Box>
          
          {isEditing ? (
            <Box sx={{ display: 'flex', flexDirection: 'column', gap: 1 }}>
              <TextField
                label="Value"
                value={editedFlag.value}
                onChange={(e) => setEditedFlag({ ...editedFlag, value: e.target.value })}
                fullWidth
                size="small"
              />
              <TextField
                label="CEL Expression"
                value={editedFlag.expression || ''}
                onChange={(e) => setEditedFlag({ ...editedFlag, expression: e.target.value })}
                fullWidth
                multiline
                rows={2}
                size="small"
                placeholder="e.g., context.region == 'us-west' && context.environment == 'prod'"
              />
              <Box sx={{ display: 'flex', justifyContent: 'flex-end', gap: 1 }}>
                <Button size="small" onClick={handleCancel}>
                  Cancel
                </Button>
                <Button size="small" variant="contained" onClick={handleSave}>
                  Save
                </Button>
              </Box>
            </Box>
          ) : (
            <Box>
              <Typography variant="body2" color="text.secondary" gutterBottom>
                Value:
              </Typography>
              {formatValue(flag.value)}
              {flag.expression && (
                <>
                  <Typography variant="body2" color="text.secondary" sx={{ mt: 1 }}>
                    Expression:
                  </Typography>
                  <Typography 
                    variant="body1" 
                    sx={{ 
                      fontFamily: 'monospace',
                      backgroundColor: 'grey.50',
                      p: 1,
                      borderRadius: 1,
                      fontSize: '0.875rem',
                      border: '1px solid',
                      borderColor: 'grey.200'
                    }}
                  >
                    {flag.expression}
                  </Typography>
                </>
              )}
            </Box>
          )}
        </CardContent>
      </Card>

      <Dialog open={isDeleteDialogOpen} onClose={() => setIsDeleteDialogOpen(false)}>
        <DialogTitle>Delete Flag</DialogTitle>
        <DialogContent>
          <Typography>
            Are you sure you want to delete the flag "{flag.key}"? This action cannot be undone.
          </Typography>
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setIsDeleteDialogOpen(false)}>Cancel</Button>
          <Button onClick={handleDelete} color="error" variant="contained">
            Delete
          </Button>
        </DialogActions>
      </Dialog>

      <Dialog open={isTryItOutDialogOpen} onClose={() => setIsTryItOutDialogOpen(false)}>
        <DialogTitle>Try it out - {flag.key}</DialogTitle>
        <DialogContent>
          {tryItOutError ? (
            <Alert severity="error" sx={{ mt: 1 }}>
              {tryItOutError}
            </Alert>
          ) : (
            <Box sx={{ mt: 1 }}>
              <Typography variant="body2" color="text.secondary" gutterBottom>
                Result:
              </Typography>
              {formatValue(tryItOutResult)}
            </Box>
          )}
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setIsTryItOutDialogOpen(false)}>Close</Button>
        </DialogActions>
      </Dialog>
    </>
  );
}; 