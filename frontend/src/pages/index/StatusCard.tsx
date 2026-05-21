import { useTranslation } from 'react-i18next';
import { Card, Col, Progress, Row, Tooltip } from 'antd';
import { AreaChartOutlined } from '@ant-design/icons';

import { CPUFormatter, SizeFormatter } from '@/utils';
import type { Status } from '@/models/status';
import './StatusCard.css';

interface StatusCardProps {
  status: Status;
  isMobile: boolean;
}

const TRAIL_COLOR = 'rgba(128, 128, 128, 0.25)';

export default function StatusCard({ status, isMobile }: StatusCardProps) {
  const { t } = useTranslation();
  const gaugeSize = isMobile ? 60 : 70;

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
                trailColor={TRAIL_COLOR}
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
                trailColor={TRAIL_COLOR}
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
                trailColor={TRAIL_COLOR}
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
                trailColor={TRAIL_COLOR}
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
