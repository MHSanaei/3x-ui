import { useTranslation } from 'react-i18next';
import { Card, Col, Progress, Row, Tooltip } from 'antd';
import { AreaChartOutlined } from '@ant-design/icons';

import { CPUFormatter, SizeFormatter } from '@/utils';
import { useTheme } from '@/hooks/useTheme';
import type { Status } from '@/models/status';
import './StatusCard.css';

interface StatusCardProps {
  status: Status;
  isMobile: boolean;
}

export default function StatusCard({ status, isMobile }: StatusCardProps) {
  const { t } = useTranslation();
  const { isDark, isUltra } = useTheme();
  const gaugeSize = isMobile ? 60 : 90;
  const strokeWidth = isMobile ? 7 : 5;
  const railColor = isDark
    ? isUltra ? 'rgba(255, 255, 255, 0.1)' : 'rgba(255, 255, 255, 0.16)'
    : 'rgba(0, 0, 0, 0.08)';

  return (
    <Card hoverable className="status-card">
      <Row gutter={[0, isMobile ? 16 : 0]}>
        <Col xs={24} md={12}>
          <Row>
            <Col span={12} className="text-center">
              <Progress
                type="dashboard"
                status="normal"
                strokeColor={status.cpu.color}
                railColor={railColor}
                strokeWidth={strokeWidth}
                percent={status.cpu.percent}
                size={gaugeSize}
              />
              <div>
                <b>{t('pages.index.cpu')}:</b> {CPUFormatter.cpuCoreFormat(status.cpuCores)}
                <Tooltip
                  title={
                    <>
                      <div>
                        <b>{t('pages.index.logicalProcessors')}:</b> {status.logicalPro}
                      </div>
                      <div>
                        <b>{t('pages.index.frequency')}:</b>{' '}
                        {CPUFormatter.cpuSpeedFormat(status.cpuSpeedMhz)}
                      </div>
                    </>
                  }
                >
                  <AreaChartOutlined />
                </Tooltip>
              </div>
            </Col>

            <Col span={12} className="text-center">
              <Progress
                type="dashboard"
                status="normal"
                strokeColor={status.mem.color}
                railColor={railColor}
                strokeWidth={strokeWidth}
                percent={status.mem.percent}
                size={gaugeSize}
              />
              <div>
                <b>{t('pages.index.memory')}:</b> {SizeFormatter.sizeFormat(status.mem.current)} /{' '}
                {SizeFormatter.sizeFormat(status.mem.total)}
              </div>
            </Col>
          </Row>
        </Col>

        <Col xs={24} md={12}>
          <Row>
            <Col span={12} className="text-center">
              <Progress
                type="dashboard"
                status="normal"
                strokeColor={status.swap.color}
                railColor={railColor}
                strokeWidth={strokeWidth}
                percent={status.swap.percent}
                size={gaugeSize}
              />
              <div>
                <b>{t('pages.index.swap')}:</b> {SizeFormatter.sizeFormat(status.swap.current)} /{' '}
                {SizeFormatter.sizeFormat(status.swap.total)}
              </div>
            </Col>

            <Col span={12} className="text-center">
              <Progress
                type="dashboard"
                status="normal"
                strokeColor={status.disk.color}
                railColor={railColor}
                strokeWidth={strokeWidth}
                percent={status.disk.percent}
                size={gaugeSize}
              />
              <div>
                <b>{t('pages.index.storage')}:</b> {SizeFormatter.sizeFormat(status.disk.current)} /{' '}
                {SizeFormatter.sizeFormat(status.disk.total)}
              </div>
            </Col>
          </Row>
        </Col>
      </Row>
    </Card>
  );
}
