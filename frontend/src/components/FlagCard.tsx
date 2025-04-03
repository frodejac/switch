import { useState } from 'react';
import {
  Card,
  CardContent,
  CardActions,
  Typography,
  TextField,
  Button,
  Box,
  Switch,
  FormControlLabel,
} from '@mui/material';
import { FeatureFlag } from '../types/flag';

interface FlagCardProps {
  flag: FeatureFlag;
  onUpdate: (key: string, flag: Partial<FeatureFlag>) => void;
}

export const FlagCard = ({ flag, onUpdate }: FlagCardProps) => {
  const [isEditing, setIsEditing] = useState(false);
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

  return (
    <Card sx={{ mb: 2 }}>
      <CardContent>
        <Typography variant="h6" gutterBottom>
          {flag.key}
        </Typography>
        {isEditing ? (
          <Box sx={{ display: 'flex', flexDirection: 'column', gap: 2 }}>
            <TextField
              label="Value"
              value={editedFlag.value}
              onChange={(e) => setEditedFlag({ ...editedFlag, value: e.target.value })}
              fullWidth
            />
            <TextField
              label="CEL Expression"
              value={editedFlag.expression || ''}
              onChange={(e) => setEditedFlag({ ...editedFlag, expression: e.target.value })}
              fullWidth
              multiline
              rows={2}
              placeholder="e.g., context.region == 'us-west' && context.environment == 'prod'"
            />
          </Box>
        ) : (
          <Box>
            <Typography variant="body1" gutterBottom>
              Value: {JSON.stringify(flag.value)}
            </Typography>
            {flag.expression && (
              <Typography variant="body2" color="text.secondary">
                Expression: {flag.expression}
              </Typography>
            )}
          </Box>
        )}
      </CardContent>
      <CardActions>
        {isEditing ? (
          <>
            <Button size="small" onClick={handleSave}>
              Save
            </Button>
            <Button size="small" onClick={handleCancel}>
              Cancel
            </Button>
          </>
        ) : (
          <Button size="small" onClick={() => setIsEditing(true)}>
            Edit
          </Button>
        )}
      </CardActions>
    </Card>
  );
}; 