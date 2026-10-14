import { useState } from 'react';
import {
  Button,
  Card,
  Col,
  Divider,
  Drawer,
  Radio,
  Row,
  Slider,
  Space,
  Tag,
  Typography,
  Upload,
  message,
} from 'antd';
import { CheckOutlined, DownloadOutlined, ReloadOutlined, UploadOutlined } from '@ant-design/icons';
import type { UploadProps } from 'antd';
import { ThemeTokenSchema, type ThemeTokens } from '@/schemas/forms/theme';
import { BUILTIN_PRESETS, PRESET_ORDER } from '@/lib/themes/presets';
import { useTheme } from '@/hooks/useTheme';

const { Text, Title } = Typography;

interface AppearanceDrawerProps {
  open: boolean;
  onClose: () => void;
}

export default function AppearanceDrawer({ open, onClose }: AppearanceDrawerProps) {
  const { activeTheme, setThemeId, setCustomTheme, resetToDefault } = useTheme();

  const [activePresetId, setActivePresetId] = useState<string>(activeTheme.id);

  const handleSelectPreset = (presetId: string) => {
    setActivePresetId(presetId);
    setThemeId(presetId);
    message.success(`Applied ${BUILTIN_PRESETS[presetId]?.name || presetId}`);
  };

  const handleColorChange = (color: string) => {
    const updated: ThemeTokens = {
      ...activeTheme,
      id: 'custom',
      name: 'Custom Theme',
      colorPrimary: color,
    };
    setActivePresetId('custom');
    setCustomTheme(updated);
  };

  const handleRadiusChange = (radius: number) => {
    const updated: ThemeTokens = {
      ...activeTheme,
      id: 'custom',
      name: 'Custom Theme',
      borderRadius: radius,
    };
    setActivePresetId('custom');
    setCustomTheme(updated);
  };

  const handleDensityChange = (density: 'default' | 'compact') => {
    const updated: ThemeTokens = {
      ...activeTheme,
      id: 'custom',
      name: 'Custom Theme',
      density,
    };
    setActivePresetId('custom');
    setCustomTheme(updated);
  };

  const handleCardStyleChange = (cardStyle: 'modern' | 'flat') => {
    const updated: ThemeTokens = {
      ...activeTheme,
      id: 'custom',
      name: 'Custom Theme',
      cardStyle,
    };
    setActivePresetId('custom');
    setCustomTheme(updated);
  };

  const exportThemeJson = () => {
    const dataStr =
      'data:text/json;charset=utf-8,' + encodeURIComponent(JSON.stringify(activeTheme, null, 2));
    const dlAnchor = document.createElement('a');
    dlAnchor.setAttribute('href', dataStr);
    dlAnchor.setAttribute('download', `3x-ui-theme-${activeTheme.id}.json`);
    dlAnchor.click();
    dlAnchor.remove();
    message.success('Theme exported successfully');
  };

  const uploadProps: UploadProps = {
    accept: '.json',
    showUploadList: false,
    beforeUpload: (file) => {
      const reader = new FileReader();
      reader.onload = (e) => {
        try {
          const json = JSON.parse(e.target?.result as string);
          const parsed = ThemeTokenSchema.parse(json);
          setActivePresetId(parsed.id);
          setCustomTheme(parsed);
          message.success(`Imported and applied theme "${parsed.name}"`);
        } catch (err: unknown) {
          const msg = err instanceof Error ? err.message : 'Invalid format';
          message.error(`Failed to import theme: ${msg}`);
        }
      };
      reader.readAsText(file);
      return false;
    },
  };

  return (
    <Drawer
      title="Appearance & Themes"
      placement="right"
      width={440}
      open={open}
      onClose={onClose}
      extra={
        <Button
          icon={<ReloadOutlined />}
          size="small"
          onClick={() => {
            resetToDefault();
            setActivePresetId('default-dark');
            message.info('Reset to default dark theme');
          }}
        >
          Reset
        </Button>
      }
    >
      <Space direction="vertical" size="large" style={{ width: '100%' }}>
        {/* Presets List */}
        <div>
          <Title level={5} style={{ marginBottom: 12 }}>
            Theme Presets
          </Title>
          <Row gutter={[12, 12]}>
            {PRESET_ORDER.map((id) => {
              const preset = BUILTIN_PRESETS[id];
              const isSelected = activePresetId === id;
              return (
                <Col span={12} key={id}>
                  <Card
                    hoverable
                    size="small"
                    style={{
                      border: isSelected
                        ? `2px solid ${preset.colorPrimary}`
                        : `1px solid ${preset.colorBorder}`,
                      background: preset.colorBgContainer,
                      cursor: 'pointer',
                      borderRadius: preset.borderRadius,
                    }}
                    onClick={() => handleSelectPreset(id)}
                  >
                    <Space orientation="vertical" size={4} style={{ width: '100%' }}>
                      <div
                        style={{
                          display: 'flex',
                          justifyContent: 'space-between',
                          alignItems: 'center',
                        }}
                      >
                        <Text strong style={{ color: preset.colorText || '#fff', fontSize: 13 }}>
                          {preset.name}
                        </Text>
                        {isSelected && <CheckOutlined style={{ color: preset.colorPrimary }} />}
                      </div>

                      <div style={{ display: 'flex', gap: 6, marginTop: 4 }}>
                        <div
                          style={{
                            width: 18,
                            height: 18,
                            borderRadius: '50%',
                            background: preset.colorPrimary,
                            border: '1px solid rgba(255,255,255,0.2)',
                          }}
                        />
                        <div
                          style={{
                            width: 18,
                            height: 18,
                            borderRadius: '50%',
                            background: preset.colorBgBase,
                            border: '1px solid rgba(255,255,255,0.2)',
                          }}
                        />
                        <div
                          style={{
                            width: 18,
                            height: 18,
                            borderRadius: '50%',
                            background: preset.colorBgElevated,
                            border: '1px solid rgba(255,255,255,0.2)',
                          }}
                        />
                        {preset.density === 'compact' && (
                          <Tag color="blue" style={{ fontSize: 10, padding: '0 4px', margin: 0 }}>
                            Compact
                          </Tag>
                        )}
                      </div>
                    </Space>
                  </Card>
                </Col>
              );
            })}
          </Row>
        </div>

        <Divider style={{ margin: '8px 0' }} />

        {/* Customization Controls */}
        <div>
          <Title level={5} style={{ marginBottom: 12 }}>
            Custom Adjustments
          </Title>

          <Space orientation="vertical" size="middle" style={{ width: '100%' }}>
            <div>
              <Text type="secondary" style={{ display: 'block', marginBottom: 6 }}>
                Primary Accent Color
              </Text>
              <input
                type="color"
                value={activeTheme.colorPrimary}
                onChange={(e) => handleColorChange(e.target.value)}
                style={{
                  width: '100%',
                  height: 36,
                  padding: 2,
                  borderRadius: 6,
                  cursor: 'pointer',
                  border: '1px solid rgba(255,255,255,0.15)',
                  background: 'transparent',
                }}
              />
            </div>

            <div>
              <div style={{ display: 'flex', justifyContent: 'space-between' }}>
                <Text type="secondary">Corner Radius</Text>
                <Text strong>{activeTheme.borderRadius}px</Text>
              </div>
              <Slider
                min={0}
                max={16}
                value={activeTheme.borderRadius}
                onChange={handleRadiusChange}
              />
            </div>

            <div>
              <Text type="secondary" style={{ display: 'block', marginBottom: 6 }}>
                Spacing Density
              </Text>
              <Radio.Group
                value={activeTheme.density}
                onChange={(e) => handleDensityChange(e.target.value)}
                buttonStyle="solid"
                style={{ width: '100%' }}
              >
                <Radio.Button value="default" style={{ width: '50%', textAlign: 'center' }}>
                  Default
                </Radio.Button>
                <Radio.Button value="compact" style={{ width: '50%', textAlign: 'center' }}>
                  Compact (2.x)
                </Radio.Button>
              </Radio.Group>
            </div>

            <div>
              <Text type="secondary" style={{ display: 'block', marginBottom: 6 }}>
                Card Style
              </Text>
              <Radio.Group
                value={activeTheme.cardStyle}
                onChange={(e) => handleCardStyleChange(e.target.value)}
                buttonStyle="solid"
                style={{ width: '100%' }}
              >
                <Radio.Button value="modern" style={{ width: '50%', textAlign: 'center' }}>
                  Modern
                </Radio.Button>
                <Radio.Button value="flat" style={{ width: '50%', textAlign: 'center' }}>
                  Flat (2.x)
                </Radio.Button>
              </Radio.Group>
            </div>
          </Space>
        </div>

        <Divider style={{ margin: '8px 0' }} />

        {/* Import & Export */}
        <div>
          <Title level={5} style={{ marginBottom: 12 }}>
            Import / Export Theme
          </Title>
          <Space orientation="horizontal" style={{ width: '100%' }}>
            <Button icon={<DownloadOutlined />} onClick={exportThemeJson}>
              Export JSON
            </Button>
            <Upload {...uploadProps}>
              <Button icon={<UploadOutlined />}>Import JSON</Button>
            </Upload>
          </Space>
        </div>
      </Space>
    </Drawer>
  );
}
