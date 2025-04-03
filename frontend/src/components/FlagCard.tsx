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
    <Card sx={{ mb: 1.5, boxShadow: 1 }}>
      <CardContent sx={{ p: 2 }}>
        <Box sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start', mb: 1 }}>
          <Typography variant="h6" sx={{ fontWeight: 600 }}>
            {flag.key}
          </Typography>
          <Button size="small" variant="outlined" onClick={() => setIsEditing(true)}>
            Edit
          </Button>
        </Box>
        
        {isEditing ? (
          <Box sx={{ display: 'flex', flexDirection: 'column', gap: 1.5 }}>
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
          <Box sx={{ display: 'flex', flexDirection: 'column', gap: 1 }}>
            {!flag.expression ? (
              <Box>
                <Typography variant="subtitle2" color="text.secondary" gutterBottom>
                  Value
                </Typography>
                {formatValue(flag.value)}
              </Box>
            ) : (
              <Box>
                <Typography variant="subtitle2" color="text.secondary" gutterBottom>
                  Expression
                </Typography>
                <Typography
                  variant="body2"
                  sx={{
                    backgroundColor: 'grey.100',
                    p: 1,
                    borderRadius: 1,
                    fontFamily: 'monospace',
                  }}
                >
                  {flag.expression}
                </Typography>
              </Box>
            )}
          </Box>
        )}
      </CardContent>
    </Card>
  );
}; 