import { useEffect, useMemo, useState } from 'react';
import { useLocation, useNavigate } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import { Alert, Button, Card, Col, ConfigProvider, Empty, Form, Layout, Result, Row, Space, Spin, Statistic, Table, Tabs, Tag, Typography, Upload, message } from 'antd';
import type { ColumnsType } from 'antd/es/table';
import type { UploadFile } from 'antd/es/upload/interface';
import {
  ApiOutlined,
  AppstoreAddOutlined,
  CodeOutlined,
  CopyOutlined,
  FileZipOutlined,
  PlusCircleOutlined,
  ReadOutlined,
  SafetyCertificateOutlined,
  UploadOutlined,
} from '@ant-design/icons';

import { usePluginsQuery } from '@/api/queries/usePluginsQuery';
import type { PluginRecord } from '@/schemas/api/plugin';
import { useTheme } from '@/hooks/useTheme';
import { useMediaQuery } from '@/hooks/useMediaQuery';
import AppSidebar from '@/layouts/AppSidebar';
import { ClipboardManager, HttpUtil } from '@/utils';
import { setMessageInstance } from '@/utils/messageBus';
import './PluginsPage.css';

function prettyJson(value: unknown): string {
  return JSON.stringify(value, null, 2);
}

export default function PluginsPage() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const { hash } = useLocation();
  const { isDark, isUltra, antdThemeConfig } = useTheme();
  const { isMobile } = useMediaQuery();
  const [messageApi, messageContextHolder] = message.useMessage();
  useEffect(() => { setMessageInstance(messageApi); }, [messageApi]);
  const { data, isFetching, isError, error, refetch } = usePluginsQuery();
  const [fileList, setFileList] = useState<UploadFile[]>([]);
  const [installing, setInstalling] = useState(false);

  const activeTab = hash === '#add' ? 'add' : hash === '#guide' ? 'guide' : 'manage';
  useEffect(() => {
    if (!hash) navigate('/plugins#manage', { replace: true });
  }, [hash, navigate]);

  const pageClass = useMemo(() => {
    const classes = ['plugins-page'];
    if (isDark) classes.push('is-dark');
    if (isUltra) classes.push('is-ultra');
    return classes.join(' ');
  }, [isDark, isUltra]);

  const installed = data?.installed ?? [];
  const capabilities = data?.capabilities;
  const template = data?.template;
  const templateJson = useMemo(() => prettyJson(template ?? {}), [template]);

  const columns = useMemo<ColumnsType<PluginRecord>>(() => [
    {
      title: t('pages.plugins.fields.name'),
      dataIndex: 'name',
      key: 'name',
      render: (_, record) => (
        <Space direction="vertical" size={0}>
          <Typography.Text strong>{record.name}</Typography.Text>
          <Typography.Text type="secondary">{record.id}</Typography.Text>
        </Space>
      ),
    },
    {
      title: t('pages.plugins.fields.version'),
      dataIndex: 'version',
      key: 'version',
      width: 120,
      render: (version: string) => <Tag>{version}</Tag>,
    },
    {
      title: t('status'),
      dataIndex: 'status',
      key: 'status',
      width: 140,
      render: (status: string, record) => (
        <Tag color={record.enabled ? 'green' : 'default'}>{status || t('disabled')}</Tag>
      ),
    },
    {
      title: t('pages.plugins.fields.author'),
      dataIndex: 'author',
      key: 'author',
      width: 180,
    },
  ], [t]);

  const installPlugin = async () => {
    const file = fileList[0]?.originFileObj;
    if (!file) {
      messageApi.warning(t('pages.plugins.upload.selectFile'));
      return;
    }
    const formData = new FormData();
    formData.append('plugin', file);
    setInstalling(true);
    try {
      const msg = await HttpUtil.post('/panel/api/plugins/install', formData, {
        headers: { 'Content-Type': 'multipart/form-data' },
      });
      if (msg?.success) {
        messageApi.success(t('pages.plugins.toasts.installed'));
        setFileList([]);
        await refetch();
        navigate('/plugins#manage');
      }
    } finally {
      setInstalling(false);
    }
  };

  return (
    <ConfigProvider theme={antdThemeConfig}>
      {messageContextHolder}
      <Layout className={pageClass}>
        <AppSidebar />
        <Layout className="content-shell">
          <Layout.Content id="content-layout" className="content-area">
            <Spin spinning={isFetching && !data} delay={200} size="large">
              {isError ? (
                <Result
                  status="error"
                  title={t('somethingWentWrong')}
                  subTitle={(error as Error)?.message}
                  extra={<Button type="primary" loading={isFetching} onClick={() => refetch()}>{t('refresh')}</Button>}
                />
              ) : (
                <Row gutter={[isMobile ? 8 : 16, isMobile ? 8 : 12]}>
                  <Col span={24}>
                    <Card size="small" className="plugins-summary">
                      <Row gutter={[16, 12]}>
                        <Col xs={8}>
                          <Statistic title={t('pages.plugins.summary.installed')} value={installed.length} prefix={<AppstoreAddOutlined />} />
                        </Col>
                        <Col xs={8}>
                          <Statistic title={t('pages.plugins.summary.hooks')} value={capabilities?.hooks.length ?? 0} prefix={<ApiOutlined />} />
                        </Col>
                        <Col xs={8}>
                          <Statistic title={t('pages.plugins.summary.permissions')} value={capabilities?.permissions.length ?? 0} prefix={<SafetyCertificateOutlined />} />
                        </Col>
                      </Row>
                    </Card>
                  </Col>

                  <Col span={24}>
                    <div className="plugins-workspace">
                      <Tabs
                        activeKey={activeTab}
                        onChange={(key) => navigate(`/plugins#${key}`)}
                        items={[
                          {
                            key: 'manage',
                            icon: <AppstoreAddOutlined />,
                            label: t('pages.plugins.manage'),
                            children: (
                              <div className="plugins-tab-panel">
                                <Table
                                  rowKey="id"
                                  size="small"
                                  columns={columns}
                                  dataSource={installed}
                                  pagination={false}
                                  scroll={{ x: 720 }}
                                  locale={{ emptyText: <Empty description={t('pages.plugins.empty')} /> }}
                                />
                              </div>
                            ),
                          },
                          {
                            key: 'add',
                            icon: <PlusCircleOutlined />,
                            label: t('pages.plugins.addNew'),
                            children: (
                              <div className="plugins-tab-panel">
                                <Card size="small" title={<Space><FileZipOutlined />{t('pages.plugins.upload.title')}</Space>}>
                                  <Form layout="vertical" onFinish={installPlugin}>
                                    <Form.Item
                                      label={t('pages.plugins.upload.file')}
                                      required
                                      extra={t('pages.plugins.upload.hint')}
                                    >
                                      <Upload.Dragger
                                        accept=".zip,application/zip,application/x-zip-compressed"
                                        maxCount={1}
                                        fileList={fileList}
                                        beforeUpload={(file) => {
                                          const isZip = file.name.toLowerCase().endsWith('.zip');
                                          if (!isZip) {
                                            messageApi.error(t('pages.plugins.upload.zipOnly'));
                                            return Upload.LIST_IGNORE;
                                          }
                                          setFileList([{
                                            uid: file.uid,
                                            name: file.name,
                                            status: 'done',
                                            originFileObj: file,
                                          }]);
                                          return false;
                                        }}
                                        onRemove={() => {
                                          setFileList([]);
                                        }}
                                      >
                                        <p className="ant-upload-drag-icon"><FileZipOutlined /></p>
                                        <p className="ant-upload-text">{t('pages.plugins.upload.dropTitle')}</p>
                                        <p className="ant-upload-hint">{t('pages.plugins.upload.dropHint')}</p>
                                      </Upload.Dragger>
                                    </Form.Item>
                                    <Button
                                      type="primary"
                                      htmlType="submit"
                                      icon={<UploadOutlined />}
                                      loading={installing}
                                      disabled={fileList.length === 0}
                                    >
                                      {t('pages.plugins.upload.install')}
                                    </Button>
                                  </Form>
                                </Card>
                              </div>
                            ),
                          },
                          {
                            key: 'guide',
                            icon: <ReadOutlined />,
                            label: t('pages.plugins.guide'),
                            children: (
                              <div className="plugins-tab-panel">
                                <Alert
                                  type="info"
                                  showIcon
                                  message={t('pages.plugins.contractTitle')}
                                  description={t('pages.plugins.contractHint')}
                                />

                                <Row gutter={[16, 16]} className="plugins-contract-grid">
                                  <Col xs={24} lg={8}>
                                    <Card size="small" title={t('pages.plugins.capabilities.runtimes')}>
                                      <Space wrap>{capabilities?.runtimes.map((item) => <Tag key={item}>{item}</Tag>)}</Space>
                                    </Card>
                                  </Col>
                                  <Col xs={24} lg={8}>
                                    <Card size="small" title={t('pages.plugins.capabilities.hooks')}>
                                      <Space wrap>{capabilities?.hooks.map((item) => <Tag key={item}>{item}</Tag>)}</Space>
                                    </Card>
                                  </Col>
                                  <Col xs={24} lg={8}>
                                    <Card size="small" title={t('pages.plugins.capabilities.permissions')}>
                                      <Space wrap>{capabilities?.permissions.map((item) => <Tag key={item}>{item}</Tag>)}</Space>
                                    </Card>
                                  </Col>
                                </Row>

                                <Card
                                  size="small"
                                  title={<Space><CodeOutlined />{t('pages.plugins.manifestTemplate')}</Space>}
                                  extra={(
                                    <Button
                                      size="small"
                                      icon={<CopyOutlined />}
                                      onClick={() => { void ClipboardManager.copyText(templateJson); }}
                                    >
                                      {t('copy')}
                                    </Button>
                                  )}
                                >
                                  <pre className="plugin-manifest-code">{templateJson}</pre>
                                </Card>
                              </div>
                            ),
                          },
                        ]}
                      />
                    </div>
                  </Col>
                </Row>
              )}
            </Spin>
          </Layout.Content>
        </Layout>
      </Layout>
    </ConfigProvider>
  );
}
