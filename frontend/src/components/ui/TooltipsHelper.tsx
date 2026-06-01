import {useTranslation} from 'react-i18next';
import {Tooltip} from 'antd';
import {QuestionCircleOutlined} from '@ant-design/icons';

export function LabelWithTooltip({labelKey, tooltipKey}: {
  labelKey: string;
  tooltipKey: string;
}) {
  const {t} = useTranslation();
  
  return (
    <Tooltip title={t(tooltipKey)}>
      {t(labelKey)} <QuestionCircleOutlined/>
    </Tooltip>
  );
}

export function LabelWithOnePerLineTooltip({labelKey}: {
  labelKey: string;
}) {
  
  return <LabelWithTooltip
    labelKey={labelKey}
    tooltipKey="pages.xray.rules.onePerLine"
  />
}