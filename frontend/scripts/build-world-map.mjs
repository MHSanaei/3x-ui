// Regenerates src/pages/logs/worldMapData.json, which is committed so the panel needs no
// map dependency at runtime. Natural Earth data is public domain. Run it from a scratch folder:
//   npm i world-atlas@2.0.2 d3-geo@3 topojson-client@3 i18n-iso-countries@7
//   node <repo>/frontend/scripts/build-world-map.mjs <scratch folder> <repo>/frontend/src/pages/logs/worldMapData.json
import { readFileSync, writeFileSync } from 'node:fs';
import { createRequire } from 'node:module';
import { join } from 'node:path';

const [scratch, output] = process.argv.slice(2);
const require = createRequire(join(scratch, 'package.json'));
const { geoNaturalEarth1, geoPath, geoCentroid } = await import(
  require.resolve('d3-geo').replace(/\\/g, '/').replace(/^([A-Za-z]):/, 'file:///$1:')
);
const { feature } = require('topojson-client');
const iso = require('i18n-iso-countries');

const atlas = (name) =>
  JSON.parse(readFileSync(join(scratch, 'node_modules/world-atlas', name), 'utf8'));
const WIDTH = 960;

// Natural Earth draws these apart with no ISO numeric id; the databases know them by these codes.
const CODE_BY_NAME = { 'N. Cyprus': 'CY', Somaliland: 'SO', Kosovo: 'XK' };
const codeOf = (country) =>
  CODE_BY_NAME[country.properties.name] ?? iso.numericToAlpha2(country.id);

// Places geoip.dat lists that Natural Earth folds into a parent or omits, as [lat, lon].
const EXTRA_MARKERS = {
  AQ: [-58, -30],
  BQ: [12.18, -68.25],
  BV: [-54.42, 3.35],
  CC: [-12.16, 96.87],
  CX: [-10.45, 105.69],
  GF: [3.93, -53.13],
  GI: [36.14, -5.35],
  GP: [16.27, -61.55],
  MQ: [14.64, -61.02],
  RE: [-21.11, 55.54],
  SJ: [78.2, 15.6],
  TK: [-9.2, -171.85],
  TV: [-7.11, 177.65],
  UM: [19.28, 166.65],
  YT: [-12.83, 45.17],
};

const small = feature(atlas('countries-110m.json'), 'countries');
small.features = small.features.filter((country) => country.properties.name !== 'Antarctica');
const detailed = feature(atlas('countries-50m.json'), 'countries');

const projection = geoNaturalEarth1().fitWidth(WIDTH, small);
const [[, top], [, bottom]] = geoPath(projection).bounds(small);
const [tx, ty] = projection.translate();
projection.translate([tx, ty - top + 2]);
const path = geoPath(projection).digits(1);
const point = ([lat, lon]) => projection([lon, lat]).map((value) => Math.round(value * 10) / 10);

const shapes = {};
for (const country of small.features) {
  const code = codeOf(country);
  if (!code) throw new Error(`no code for ${country.properties.name}`);
  shapes[code] = (shapes[code] ?? '') + path(country);
}

const markers = {};
for (const country of detailed.features) {
  const code = codeOf(country);
  if (code && !shapes[code]) {
    const [lon, lat] = geoCentroid(country);
    markers[code] = point([lat, lon]);
  }
}
for (const [code, position] of Object.entries(EXTRA_MARKERS)) {
  if (!shapes[code]) markers[code] = point(position);
}

const height = Math.ceil(bottom - top + 4);
writeFileSync(output, `${JSON.stringify({ width: WIDTH, height, shapes, markers })}\n`);
console.log(
  `shapes=${Object.keys(shapes).length} markers=${Object.keys(markers).length} height=${height}`,
);
