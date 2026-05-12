<script setup>
import { computed } from 'vue';
import { DeleteOutlined, PlusOutlined, ReloadOutlined } from '@ant-design/icons-vue';
import { RandomUtil } from '@/utils';
import { Protocols } from '@/models/inbound.js';

// Mirrors web/html/form/stream/stream_finalmask.html. Used by both the
// inbound and outbound modals — they share the same StreamSettings
// shape (`stream.finalmask`, `stream.addTcpMask()`, etc.) so a single
// component handles both. The host modal passes its protocol through
// so we know whether to show only the Hysteria-specific UDP types.
const props = defineProps({
  stream: { type: Object, required: true },
  protocol: { type: String, default: '' },
});

const isHysteria = computed(() => props.protocol === Protocols.HYSTERIA);
const network = computed(() => props.stream?.network || '');

const showTcp = computed(() => ['raw', 'tcp', 'httpupgrade', 'ws', 'grpc', 'xhttp'].includes(network.value));
const showUdp = computed(() => isHysteria.value || network.value === 'kcp');
const showQuic = computed(() => isHysteria.value || network.value === 'xhttp');

// Reset the per-row settings shape when the user picks a different
// type — mirrors the legacy `mask._getDefaultSettings(type, {})` call.
function changeMaskType(mask, type) {
  mask.type = type;
  mask.settings = mask._getDefaultSettings(type, {});
}

// Special case from the legacy form: switching a UDP mask to xdns
// shrinks the kcp MTU; everything else needs the default 1350.
function changeUdpMaskType(mask, type) {
  changeMaskType(mask, type);
  if (network.value === 'kcp' && props.stream.kcp) {
    props.stream.kcp.mtu = type === 'xdns' ? 900 : 1350;
  }
}

// header-custom and noise rows share the same per-item shape — the
// type select rewires the packet field. Pulled out so the click
// handlers in the template stay readable.
function changeItemType(item, type) {
  item.type = type;
  if (type === 'base64') item.packet = RandomUtil.randomBase64();
  else if (type === 'array') { item.rand = 0; item.packet = []; }
  else item.packet = '';
}

function addUdpMaskWithDefault() {
  const def = isHysteria.value ? 'salamander' : 'mkcp-aes128gcm';
  props.stream.addUdpMask(def);
}

function newClientServerItem() {
  return { delay: 0, rand: 0, randRange: '0-255', type: 'array', packet: [] };
}

function newUdpClientServerItem() {
  return { rand: 0, randRange: '0-255', type: 'array', packet: [] };
}

function newNoiseItem() {
  return { rand: '1-8192', randRange: '0-255', type: 'array', packet: [], delay: '10-20' };
}
</script>

<template>
  <a-form v-if="showTcp || showUdp || showQuic" :colon="false" :label-col="{ md: { span: 8 } }"
    :wrapper-col="{ md: { span: 14 } }">
    <!-- ============================== TCP MASKS ============================== -->
    <template v-if="showTcp">
      <a-form-item label="TCP Masks">
        <a-button type="primary" size="small" @click="stream.addTcpMask('fragment')">
          <template #icon>
            <PlusOutlined />
          </template>
        </a-button>
      </a-form-item>

      <template v-for="(mask, mIdx) in (stream.finalmask.tcp || [])" :key="`tcp-${mIdx}`">
        <a-divider :style="{ margin: '0' }">
          TCP Mask {{ mIdx + 1 }}
          <DeleteOutlined :style="{ color: 'rgb(255, 77, 79)', cursor: 'pointer', marginLeft: '8px' }"
            @click="stream.delTcpMask(mIdx)" />
        </a-divider>

        <a-form-item label="Type">
          <a-select :value="mask.type" @change="(t) => changeMaskType(mask, t)">
            <a-select-option value="fragment">Fragment</a-select-option>
            <a-select-option value="header-custom">Header Custom</a-select-option>
            <a-select-option value="sudoku">Sudoku</a-select-option>
          </a-select>
        </a-form-item>

        <!-- Fragment -->
        <template v-if="mask.type === 'fragment'">
          <a-form-item label="Packets">
            <a-select v-model:value="mask.settings.packets">
              <a-select-option value="tlshello">tlshello</a-select-option>
              <a-select-option value="1-3">1-3</a-select-option>
              <a-select-option value="1-5">1-5</a-select-option>
            </a-select>
          </a-form-item>
          <a-form-item label="Length">
            <a-input v-model:value="mask.settings.length" placeholder="e.g. 100-200" />
          </a-form-item>
          <a-form-item label="Delay">
            <a-input v-model:value="mask.settings.delay" placeholder="e.g. 10-20" />
          </a-form-item>
          <a-form-item label="Max Split">
            <a-input v-model:value="mask.settings.maxSplit" placeholder="e.g. 3-6" />
          </a-form-item>
        </template>

        <!-- Sudoku -->
        <template v-if="mask.type === 'sudoku'">
          <a-form-item label="Password">
            <a-input v-model:value="mask.settings.password" placeholder="Obfuscation password" />
          </a-form-item>
          <a-form-item label="ASCII">
            <a-input v-model:value="mask.settings.ascii" placeholder="ASCII" />
          </a-form-item>
          <a-form-item label="Custom Table">
            <a-input v-model:value="mask.settings.customTable" placeholder="Custom Table" />
          </a-form-item>
          <a-form-item label="Custom Tables">
            <a-input v-model:value="mask.settings.customTables" placeholder="Custom Tables" />
          </a-form-item>
          <a-form-item label="Padding Min">
            <a-input-number v-model:value="mask.settings.paddingMin" :min="0" />
          </a-form-item>
          <a-form-item label="Padding Max">
            <a-input-number v-model:value="mask.settings.paddingMax" :min="0" />
          </a-form-item>
        </template>

        <!-- Header Custom — clients/servers as 2D groups -->
        <template v-if="mask.type === 'header-custom'">
          <!-- Clients -->
          <a-form-item label="Clients">
            <a-button type="primary" size="small" @click="mask.settings.clients.push([newClientServerItem()])">
              <template #icon>
                <PlusOutlined />
              </template>
            </a-button>
          </a-form-item>
          <template v-for="(group, gi) in mask.settings.clients" :key="`tcp-cg-${mIdx}-${gi}`">
            <a-divider :style="{ margin: '0' }">
              Clients Group {{ gi + 1 }}
              <DeleteOutlined :style="{ color: 'rgb(255, 77, 79)', cursor: 'pointer', marginLeft: '8px' }"
                @click="mask.settings.clients.splice(gi, 1)" />
            </a-divider>
            <template v-for="(item, ii) in group" :key="`tcp-ci-${mIdx}-${gi}-${ii}`">
              <a-form-item label="Type">
                <a-select :value="item.type" @change="(t) => changeItemType(item, t)">
                  <a-select-option value="array">Array</a-select-option>
                  <a-select-option value="str">String</a-select-option>
                  <a-select-option value="hex">Hex</a-select-option>
                  <a-select-option value="base64">Base64</a-select-option>
                </a-select>
              </a-form-item>
              <a-form-item label="Delay (ms)">
                <a-input-number v-model:value="item.delay" :min="0" />
              </a-form-item>
              <template v-if="item.type === 'array'">
                <a-form-item label="Rand">
                  <a-input-number v-model:value="item.rand" :min="0" />
                </a-form-item>
                <a-form-item label="Rand Range">
                  <a-input v-model:value="item.randRange" placeholder="0-255" />
                </a-form-item>
              </template>
              <a-form-item v-else label="Packet">
                <a-input-group v-if="item.type === 'base64'" compact>
                  <a-input v-model:value="item.packet" placeholder="binary data"
                    :style="{ width: 'calc(100% - 32px)' }" />
                  <a-button @click="item.packet = RandomUtil.randomBase64()">
                    <template #icon>
                      <ReloadOutlined />
                    </template>
                  </a-button>
                </a-input-group>
                <a-input v-else v-model:value="item.packet" placeholder="binary data" />
              </a-form-item>
            </template>
          </template>

          <!-- Servers -->
          <a-form-item label="Servers">
            <a-button type="primary" size="small" @click="mask.settings.servers.push([newClientServerItem()])">
              <template #icon>
                <PlusOutlined />
              </template>
            </a-button>
          </a-form-item>
          <template v-for="(group, gi) in mask.settings.servers" :key="`tcp-sg-${mIdx}-${gi}`">
            <a-divider :style="{ margin: '0' }">
              Servers Group {{ gi + 1 }}
              <DeleteOutlined :style="{ color: 'rgb(255, 77, 79)', cursor: 'pointer', marginLeft: '8px' }"
                @click="mask.settings.servers.splice(gi, 1)" />
            </a-divider>
            <template v-for="(item, ii) in group" :key="`tcp-si-${mIdx}-${gi}-${ii}`">
              <a-form-item label="Type">
                <a-select :value="item.type" @change="(t) => changeItemType(item, t)">
                  <a-select-option value="array">Array</a-select-option>
                  <a-select-option value="str">String</a-select-option>
                  <a-select-option value="hex">Hex</a-select-option>
                  <a-select-option value="base64">Base64</a-select-option>
                </a-select>
              </a-form-item>
              <a-form-item label="Delay (ms)">
                <a-input-number v-model:value="item.delay" :min="0" />
              </a-form-item>
              <template v-if="item.type === 'array'">
                <a-form-item label="Rand">
                  <a-input-number v-model:value="item.rand" :min="0" />
                </a-form-item>
                <a-form-item label="Rand Range">
                  <a-input v-model:value="item.randRange" placeholder="0-255" />
                </a-form-item>
              </template>
              <a-form-item v-else label="Packet">
                <a-input-group v-if="item.type === 'base64'" compact>
                  <a-input v-model:value="item.packet" placeholder="binary data"
                    :style="{ width: 'calc(100% - 32px)' }" />
                  <a-button @click="item.packet = RandomUtil.randomBase64()">
                    <template #icon>
                      <ReloadOutlined />
                    </template>
                  </a-button>
                </a-input-group>
                <a-input v-else v-model:value="item.packet" placeholder="binary data" />
              </a-form-item>
            </template>
          </template>
        </template>
      </template>
    </template>

    <!-- ============================== UDP MASKS ============================== -->
    <template v-if="showUdp">
      <a-form-item label="UDP Masks">
        <a-button type="primary" size="small" @click="addUdpMaskWithDefault">
          <template #icon>
            <PlusOutlined />
          </template>
        </a-button>
      </a-form-item>

      <template v-for="(mask, mIdx) in (stream.finalmask.udp || [])" :key="`udp-${mIdx}`">
        <a-divider :style="{ margin: '0' }">
          UDP Mask {{ mIdx + 1 }}
          <DeleteOutlined :style="{ color: 'rgb(255, 77, 79)', cursor: 'pointer', marginLeft: '8px' }"
            @click="stream.delUdpMask(mIdx)" />
        </a-divider>

        <a-form-item label="Type">
          <a-select :value="mask.type" @change="(t) => changeUdpMaskType(mask, t)">
            <template v-if="isHysteria">
              <a-select-option value="salamander">Salamander (Hysteria2)</a-select-option>
            </template>
            <template v-else>
              <a-select-option value="mkcp-aes128gcm">mKCP AES-128-GCM</a-select-option>
              <a-select-option value="header-dns">Header DNS</a-select-option>
              <a-select-option value="header-dtls">Header DTLS 1.2</a-select-option>
              <a-select-option value="header-srtp">Header SRTP</a-select-option>
              <a-select-option value="header-utp">Header uTP</a-select-option>
              <a-select-option value="header-wechat">Header WeChat Video</a-select-option>
              <a-select-option value="header-wireguard">Header WireGuard</a-select-option>
              <a-select-option value="mkcp-original">mKCP Original</a-select-option>
              <a-select-option value="xdns">xDNS</a-select-option>
              <a-select-option value="xicmp">xICMP</a-select-option>
              <a-select-option value="header-custom">Header Custom</a-select-option>
              <a-select-option value="noise">Noise</a-select-option>
            </template>
          </a-select>
        </a-form-item>

        <a-form-item v-if="['mkcp-aes128gcm', 'salamander'].includes(mask.type)" label="Password">
          <a-input v-model:value="mask.settings.password" placeholder="Obfuscation password" />
        </a-form-item>
        <a-form-item v-if="mask.type === 'header-dns'" label="Domain">
          <a-input v-model:value="mask.settings.domain" placeholder="e.g., www.example.com" />
        </a-form-item>
        <a-form-item v-if="mask.type === 'xdns'" label="Domains">
          <a-select v-model:value="mask.settings.domains" mode="tags" :style="{ width: '100%' }"
            :token-separators="[',']" placeholder="e.g., www.example.com" />
        </a-form-item>

        <!-- Noise -->
        <template v-if="mask.type === 'noise'">
          <a-form-item label="Reset">
            <a-input-number v-model:value="mask.settings.reset" :min="0" />
          </a-form-item>
          <a-form-item label="Noise">
            <a-button type="primary" size="small" @click="mask.settings.noise.push(newNoiseItem())">
              <template #icon>
                <PlusOutlined />
              </template>
            </a-button>
          </a-form-item>
          <template v-for="(n, ni) in mask.settings.noise" :key="`udp-noise-${mIdx}-${ni}`">
            <a-divider :style="{ margin: '0' }">
              Noise {{ ni + 1 }}
              <DeleteOutlined :style="{ color: 'rgb(255, 77, 79)', cursor: 'pointer', marginLeft: '8px' }"
                @click="mask.settings.noise.splice(ni, 1)" />
            </a-divider>
            <a-form-item label="Type">
              <a-select :value="n.type" @change="(t) => changeItemType(n, t)">
                <a-select-option value="array">Array</a-select-option>
                <a-select-option value="str">String</a-select-option>
                <a-select-option value="hex">Hex</a-select-option>
                <a-select-option value="base64">Base64</a-select-option>
              </a-select>
            </a-form-item>
            <template v-if="n.type === 'array'">
              <a-form-item label="Rand">
                <a-input v-model:value="n.rand" placeholder="0 or 1-8192" />
              </a-form-item>
              <a-form-item label="Rand Range">
                <a-input v-model:value="n.randRange" placeholder="0-255" />
              </a-form-item>
            </template>
            <a-form-item v-else label="Packet">
              <a-input-group v-if="n.type === 'base64'" compact>
                <a-input v-model:value="n.packet" placeholder="binary data" :style="{ width: 'calc(100% - 32px)' }" />
                <a-button @click="n.packet = RandomUtil.randomBase64()">
                  <template #icon>
                    <ReloadOutlined />
                  </template>
                </a-button>
              </a-input-group>
              <a-input v-else v-model:value="n.packet" placeholder="binary data" />
            </a-form-item>
            <a-form-item label="Delay">
              <a-input v-model:value="n.delay" placeholder="10-20" />
            </a-form-item>
          </template>
        </template>

        <!-- Header Custom (UDP) — flat client/server lists -->
        <template v-if="mask.type === 'header-custom'">
          <a-form-item label="Client">
            <a-button type="primary" size="small" @click="mask.settings.client.push(newUdpClientServerItem())">
              <template #icon>
                <PlusOutlined />
              </template>
            </a-button>
          </a-form-item>
          <template v-for="(c, ci) in mask.settings.client" :key="`udp-c-${mIdx}-${ci}`">
            <a-divider :style="{ margin: '0' }">
              Client {{ ci + 1 }}
              <DeleteOutlined :style="{ color: 'rgb(255, 77, 79)', cursor: 'pointer', marginLeft: '8px' }"
                @click="mask.settings.client.splice(ci, 1)" />
            </a-divider>
            <a-form-item label="Type">
              <a-select :value="c.type" @change="(t) => changeItemType(c, t)">
                <a-select-option value="array">Array</a-select-option>
                <a-select-option value="str">String</a-select-option>
                <a-select-option value="hex">Hex</a-select-option>
                <a-select-option value="base64">Base64</a-select-option>
              </a-select>
            </a-form-item>
            <template v-if="c.type === 'array'">
              <a-form-item label="Rand">
                <a-input-number v-model:value="c.rand" />
              </a-form-item>
              <a-form-item label="Rand Range">
                <a-input v-model:value="c.randRange" placeholder="0-255" />
              </a-form-item>
            </template>
            <a-form-item v-else label="Packet">
              <a-input-group v-if="c.type === 'base64'" compact>
                <a-input v-model:value="c.packet" placeholder="binary data" :style="{ width: 'calc(100% - 32px)' }" />
                <a-button @click="c.packet = RandomUtil.randomBase64()">
                  <template #icon>
                    <ReloadOutlined />
                  </template>
                </a-button>
              </a-input-group>
              <a-input v-else v-model:value="c.packet" placeholder="binary data" />
            </a-form-item>
          </template>

          <a-divider :style="{ margin: '0' }" />
          <a-form-item label="Server">
            <a-button type="primary" size="small" @click="mask.settings.server.push(newUdpClientServerItem())">
              <template #icon>
                <PlusOutlined />
              </template>
            </a-button>
          </a-form-item>
          <template v-for="(s, si) in mask.settings.server" :key="`udp-s-${mIdx}-${si}`">
            <a-divider :style="{ margin: '0' }">
              Server {{ si + 1 }}
              <DeleteOutlined :style="{ color: 'rgb(255, 77, 79)', cursor: 'pointer', marginLeft: '8px' }"
                @click="mask.settings.server.splice(si, 1)" />
            </a-divider>
            <a-form-item label="Type">
              <a-select :value="s.type" @change="(t) => changeItemType(s, t)">
                <a-select-option value="array">Array</a-select-option>
                <a-select-option value="str">String</a-select-option>
                <a-select-option value="hex">Hex</a-select-option>
                <a-select-option value="base64">Base64</a-select-option>
              </a-select>
            </a-form-item>
            <template v-if="s.type === 'array'">
              <a-form-item label="Rand">
                <a-input-number v-model:value="s.rand" />
              </a-form-item>
              <a-form-item label="Rand Range">
                <a-input v-model:value="s.randRange" placeholder="0-255" />
              </a-form-item>
            </template>
            <a-form-item v-else label="Packet">
              <a-input-group v-if="s.type === 'base64'" compact>
                <a-input v-model:value="s.packet" placeholder="binary data" :style="{ width: 'calc(100% - 32px)' }" />
                <a-button @click="s.packet = RandomUtil.randomBase64()">
                  <template #icon>
                    <ReloadOutlined />
                  </template>
                </a-button>
              </a-input-group>
              <a-input v-else v-model:value="s.packet" placeholder="binary data" />
            </a-form-item>
          </template>
        </template>

        <!-- xICMP -->
        <template v-if="mask.type === 'xicmp'">
          <a-form-item label="IP">
            <a-input v-model:value="mask.settings.ip" placeholder="0.0.0.0" />
          </a-form-item>
          <a-form-item label="ID">
            <a-input-number v-model:value="mask.settings.id" :min="0" />
          </a-form-item>
        </template>
      </template>
    </template>

    <!-- ============================== QUIC PARAMS ============================== -->
    <template v-if="showQuic">
      <a-form-item label="QUIC Params">
        <a-switch v-model:checked="stream.finalmask.enableQuicParams" />
      </a-form-item>
      <template v-if="stream.finalmask.enableQuicParams && stream.finalmask.quicParams">
        <a-form-item label="Congestion">
          <a-select v-model:value="stream.finalmask.quicParams.congestion">
            <a-select-option value="reno">Reno</a-select-option>
            <a-select-option value="bbr">BBR</a-select-option>
            <a-select-option value="brutal">Brutal</a-select-option>
            <a-select-option value="force-brutal">Force Brutal</a-select-option>
          </a-select>
        </a-form-item>
        <a-form-item label="Debug">
          <a-switch v-model:checked="stream.finalmask.quicParams.debug" />
        </a-form-item>
        <template v-if="['brutal', 'force-brutal'].includes(stream.finalmask.quicParams.congestion)">
          <a-form-item label="Brutal Up">
            <a-input v-model:value="stream.finalmask.quicParams.brutalUp" placeholder="65537" />
          </a-form-item>
          <a-form-item label="Brutal Down">
            <a-input v-model:value="stream.finalmask.quicParams.brutalDown" placeholder="65537" />
          </a-form-item>
        </template>
        <a-form-item label="UDP Hop">
          <a-switch v-model:checked="stream.finalmask.quicParams.hasUdpHop" />
        </a-form-item>
        <template v-if="stream.finalmask.quicParams.hasUdpHop && stream.finalmask.quicParams.udpHop">
          <a-form-item label="Hop Ports">
            <a-input v-model:value="stream.finalmask.quicParams.udpHop.ports" placeholder="e.g. 20000-50000" />
          </a-form-item>
          <a-form-item label="Hop Interval (s)">
            <a-input-number v-model:value="stream.finalmask.quicParams.udpHop.interval" :min="5" />
          </a-form-item>
        </template>
        <a-form-item label="Max Idle Timeout (s)">
          <a-input-number v-model:value="stream.finalmask.quicParams.maxIdleTimeout" :min="4" :max="120" />
        </a-form-item>
        <a-form-item label="Keep Alive Period (s)">
          <a-input-number v-model:value="stream.finalmask.quicParams.keepAlivePeriod" :min="2" :max="60" />
        </a-form-item>
        <a-form-item label="Disable Path MTU Dis">
          <a-switch v-model:checked="stream.finalmask.quicParams.disablePathMTUDiscovery" />
        </a-form-item>
        <a-form-item label="Max Incoming Streams">
          <a-input-number v-model:value="stream.finalmask.quicParams.maxIncomingStreams" :min="8"
            placeholder="1024 = default" />
        </a-form-item>
        <a-form-item label="Init Stream Window">
          <a-input-number v-model:value="stream.finalmask.quicParams.initStreamReceiveWindow" :min="16384"
            placeholder="8388608 = default" />
        </a-form-item>
        <a-form-item label="Max Stream Window">
          <a-input-number v-model:value="stream.finalmask.quicParams.maxStreamReceiveWindow" :min="16384"
            placeholder="8388608 = default" />
        </a-form-item>
        <a-form-item label="Init Conn Window">
          <a-input-number v-model:value="stream.finalmask.quicParams.initConnectionReceiveWindow" :min="16384"
            placeholder="20971520 = default" />
        </a-form-item>
        <a-form-item label="Max Conn Window">
          <a-input-number v-model:value="stream.finalmask.quicParams.maxConnectionReceiveWindow" :min="16384"
            placeholder="20971520 = default" />
        </a-form-item>
      </template>
    </template>
  </a-form>
</template>
