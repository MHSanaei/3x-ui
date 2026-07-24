import { CloudOutlined } from '@ant-design/icons';

interface CountryPillProps {
  flag: string;
  name: string;
  warp?: string;
}

export default function CountryPill({ flag, name, warp }: CountryPillProps) {
  const isWarp = !!warp && warp.toLowerCase() !== 'off';
  return (
    <span className={isWarp ? 'country-pill warp-on' : 'country-pill'}>
      {isWarp && <CloudOutlined className="warp-cloud-icon" />}
      {flag && <span>{flag}</span>}
      <span>{name}</span>
    </span>
  );
}
