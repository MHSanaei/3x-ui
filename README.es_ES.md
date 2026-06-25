[English](/README.md) | [فارسی](/README.fa_IR.md) | [العربية](/README.ar_EG.md) | [中文](/README.zh_CN.md) | [Español](/README.es_ES.md) | [Русский](/README.ru_RU.md) | [Türkçe](/README.tr_TR.md)

<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="./media/3x-ui-dark.png">
    <img alt="3x-ui" src="./media/3x-ui-light.png">
  </picture>
</p>

<p align="center">
  <a href="https://github.com/MHSanaei/3x-ui/releases"><img src="https://img.shields.io/github/v/release/mhsanaei/3x-ui" alt="Release"></a>
  <a href="https://github.com/MHSanaei/3x-ui/actions"><img src="https://img.shields.io/github/actions/workflow/status/mhsanaei/3x-ui/release.yml.svg" alt="Build"></a>
  <a href="#"><img src="https://img.shields.io/github/go-mod/go-version/mhsanaei/3x-ui.svg" alt="GO Version"></a>
  <a href="https://github.com/MHSanaei/3x-ui/releases/latest"><img src="https://img.shields.io/github/downloads/mhsanaei/3x-ui/total.svg" alt="Downloads"></a>
  <a href="https://www.gnu.org/licenses/gpl-3.0.en.html"><img src="https://img.shields.io/badge/license-GPL%20V3-blue.svg?longCache=true" alt="License"></a>
  <a href="https://pkg.go.dev/github.com/mhsanaei/3x-ui/v3"><img src="https://pkg.go.dev/badge/github.com/mhsanaei/3x-ui/v3.svg" alt="Go Reference"></a>
  <a href="https://goreportcard.com/report/github.com/mhsanaei/3x-ui/v3"><img src="https://goreportcard.com/badge/github.com/mhsanaei/3x-ui/v3" alt="Go Report Card"></a>
</p>

**3X-UI** es un panel de control web avanzado y de código abierto para gestionar servidores [Xray-core](https://github.com/XTLS/Xray-core). Ofrece una interfaz limpia y multilingüe para desplegar, configurar y monitorear una amplia gama de protocolos de proxy y VPN — desde un único VPS hasta despliegues multinodo.

Construido como un fork mejorado del proyecto X-UI original, 3X-UI añade un soporte de protocolos más amplio, mayor estabilidad, contabilidad de tráfico por cliente y muchas funciones que mejoran la experiencia de uso.

> [!IMPORTANT]
> Este proyecto está destinado únicamente al uso personal. Por favor, no lo uses para fines ilegales ni en un entorno de producción.

## Características

- **Entradas multiprotocolo** — VLESS, VMess, Trojan, Shadowsocks, WireGuard, Hysteria2, HTTP, SOCKS (Mixed), Dokodemo-door / Tunnel y TUN.
- **Transportes y seguridad modernos** — TCP (Raw), mKCP, WebSocket, gRPC, HTTPUpgrade y XHTTP, protegidos con TLS, XTLS y REALITY.
- **Fallbacks** — sirve varios protocolos en un solo puerto (p. ej. VLESS y Trojan en el 443) usando la función de fallback de Xray.
- **Gestión por cliente** — cuotas de tráfico, fechas de caducidad, límites de IP, estado en línea en tiempo real y enlaces de compartición, códigos QR y suscripciones con un solo clic.
- **Estadísticas de tráfico** — por entrada, por cliente y por salida, con controles de reinicio.
- **Soporte multinodo** — gestiona y escala a través de varios servidores desde un único panel.
- **Salida y enrutamiento** — WARP, NordVPN, reglas de enrutamiento personalizadas, balanceadores de carga y encadenamiento de proxy de salida.
- **Servidor de suscripción integrado** con múltiples formatos de salida y [plantillas de página personalizables](docs/custom-subscription-templates.md).
- **Bot de Telegram** para monitorización y gestión remotas.
- **API RESTful** con documentación Swagger dentro del panel.
- **Almacenamiento flexible** — SQLite (predeterminado) o PostgreSQL.
- **13 idiomas de interfaz** con temas oscuro y claro.
- **Integración con Fail2ban** para aplicar límites de IP por cliente.

## Capturas de pantalla

<details>
<summary>Haz clic para expandir</summary>

<picture>
  <source media="(prefers-color-scheme: dark)" srcset="./media/01-overview-dark.png">
  <img alt="Overview" src="./media/01-overview-light.png">
</picture>

<picture>
  <source media="(prefers-color-scheme: dark)" srcset="./media/02-add-inbound-dark.png">
  <img alt="Inbounds" src="./media/02-add-inbound-light.png">
</picture>

<picture>
  <source media="(prefers-color-scheme: dark)" srcset="./media/03-add-client-dark.png">
  <img alt="Add client" src="./media/03-add-client-light.png">
</picture>

<picture>
  <source media="(prefers-color-scheme: dark)" srcset="./media/05-add-nodes-dark.png">
  <img alt="Configs" src="./media/05-add-nodes-light.png">
</picture>

</details>

## Inicio Rápido

```bash
bash <(curl -Ls https://raw.githubusercontent.com/mhsanaei/3x-ui/master/install.sh)
```

Para instalar una versión específica, añade su etiqueta (p. ej. `v3.4.0`):

```bash
bash <(curl -Ls https://raw.githubusercontent.com/mhsanaei/3x-ui/master/install.sh) v3.4.0
```

Para instalar la versión **dev** continua (la última prelanzamiento por commit desde `main`, no una versión estable), pasa `dev-latest`:

```bash
bash <(curl -Ls https://raw.githubusercontent.com/mhsanaei/3x-ui/master/install.sh) dev-latest
```

Durante la instalación se generan un nombre de usuario, una contraseña y una ruta de acceso aleatorios. Tras la instalación, ejecuta `x-ui` para abrir el menú de gestión, donde puedes iniciar/detener el servicio, ver o restablecer tus credenciales de acceso, gestionar certificados SSL y mucho más.

Para la documentación completa, visita la [Wiki del proyecto](https://github.com/MHSanaei/3x-ui/wiki).

### Instalación desatendida e imágenes de nube

El instalador también se ejecuta de forma **no interactiva** para cloud-init e imágenes doradas (golden images).
Define `XUI_NONINTERACTIVE=1` (o canalízalo sin TTY) y realizará la instalación de principio a fin sin
ninguna pregunta, generando credenciales aleatorias y escribiéndolas en
`/etc/x-ui/install-result.env`. Consulta [`deploy/`](deploy/) para:

- [User-data de cloud-init](deploy/cloud-init/) — instalación desatendida en cualquier nube (Hetzner/AWS/DO/Vultr/GCP/Azure/Oracle)
- [Imagen dorada de Packer](deploy/packer/) — crea una AMI de AWS EC2 + qcow2 (amd64/arm64) con credenciales por instancia generadas en el primer arranque
- [Amazon Lightsail](deploy/lightsail/) — script de lanzamiento + constructor de snapshots reutilizable
- [Lista de verificación de AWS Marketplace](deploy/marketplace/aws/)

## Plataformas Compatibles

**Sistemas operativos:** Ubuntu, Debian, Armbian, Fedora, CentOS, RHEL, AlmaLinux, Rocky Linux, Oracle Linux, Amazon Linux, Virtuozzo, Arch, Manjaro, Parch, openSUSE (Tumbleweed / Leap), Alpine y Windows.

**Arquitecturas:** `amd64` · `386` · `arm64` (aarch64) · `armv7` · `armv6` · `armv5` · `s390x`.

## Opciones de Base de Datos

3X-UI admite dos backends, que se eligen durante la instalación:

- **SQLite** (predeterminado) — un único archivo en `/etc/x-ui/x-ui.db`. Sin configuración, ideal para despliegues pequeños y medianos.
- **PostgreSQL** — recomendado para un gran número de clientes o configuraciones multinodo. El instalador puede instalar PostgreSQL localmente por ti, o aceptar un DSN a un servidor existente.

En tiempo de ejecución, el backend se selecciona mediante variables de entorno (el instalador las escribe por ti en `/etc/default/x-ui`):

```
XUI_DB_TYPE=postgres
XUI_DB_DSN=postgres://xui:password@127.0.0.1:5432/xui?sslmode=disable
```

### Migrar una instalación de SQLite existente a PostgreSQL

```bash
x-ui migrate-db --dsn "postgres://xui:password@127.0.0.1:5432/xui?sslmode=disable"
# luego define XUI_DB_TYPE y XUI_DB_DSN en /etc/default/x-ui y reinicia:
systemctl restart x-ui
```

El archivo SQLite de origen permanece intacto; elimínalo manualmente una vez que hayas verificado el nuevo backend.

### Docker

El comando predeterminado `docker compose up -d` sigue usando SQLite. Para ejecutarlo con el servicio PostgreSQL incluido, descomenta las dos líneas de variables de entorno `XUI_DB_*` en `docker-compose.yml` e inícialo con el perfil:

```bash
docker compose --profile postgres up -d
```

La imagen incluye Fail2ban (habilitado de forma predeterminada) para aplicar **límites de IP** por cliente. Fail2ban banea a los infractores con `iptables`, lo que requiere la capacidad `NET_ADMIN`. `docker-compose.yml` ya la concede mediante `cap_add`; si en su lugar inicias el contenedor con `docker run`, añade tú mismo las capacidades, de lo contrario los baneos se registran pero nunca se aplican:

```bash
docker run -d --cap-add=NET_ADMIN --cap-add=NET_RAW ... ghcr.io/mhsanaei/3x-ui
```

## Variables de Entorno

| Variable | Descripción | Predeterminado |
| --- | --- | --- |
| `XUI_DB_TYPE` | Backend de base de datos: `sqlite` o `postgres` | `sqlite` |
| `XUI_DB_DSN` | Cadena de conexión de PostgreSQL (cuando `XUI_DB_TYPE=postgres`) | — |
| `XUI_DB_FOLDER` | Directorio del archivo de base de datos SQLite | `/etc/x-ui` |
| `XUI_DB_MAX_OPEN_CONNS` | Máximo de conexiones abiertas (pool de PostgreSQL) | — |
| `XUI_DB_MAX_IDLE_CONNS` | Máximo de conexiones inactivas (pool de PostgreSQL) | — |
| `XUI_INIT_WEB_BASE_PATH` | La ruta URI inicial para el panel web | `/` |
| `XUI_ENABLE_FAIL2BAN` | Habilitar la aplicación de límites de IP basada en Fail2ban | `true` |
| `XUI_LOG_LEVEL` | Nivel de registro (`debug`, `info`, `warning`, `error`) | `info` |
| `XUI_DEBUG` | Habilitar el modo de depuración | `false` |
| `XUI_TUNNEL_HEALTH_MONITOR` | Habilitar el monitor de salud del túnel (sondea una URL y reinicia xray tras fallos repetidos; un reinicio desconecta a todos los clientes) | `false` |
| `XUI_TUNNEL_HEALTH_PROXY` | Proxy a través del cual se envía el sondeo; apúntalo a una entrada local de xray para que el sondeo pruebe el túnel (p. ej. `socks5://127.0.0.1:1080`). Vacío significa que el sondeo solo comprueba la conectividad del host | — |
| `XUI_TUNNEL_HEALTH_URL` | URL sondeada para verificar la salud del túnel | `https://www.cloudflare.com/cdn-cgi/trace` |
| `XUI_TUNNEL_HEALTH_INTERVAL` | Intervalo entre sondeos | `30s` |
| `XUI_TUNNEL_HEALTH_TIMEOUT` | Tiempo de espera por sondeo | `10s` |
| `XUI_TUNNEL_HEALTH_FAILURES` | Fallos consecutivos antes de que se active un reinicio | `3` |
| `XUI_TUNNEL_HEALTH_COOLDOWN` | Retardo mínimo entre reinicios consecutivos | `5m` |

## Idiomas Compatibles

La interfaz del panel está disponible en 13 idiomas:

English · فارسی · العربية · 中文（简体） · 中文（繁體） · Español · Русский · Українська · Türkçe · Tiếng Việt · 日本語 · Bahasa Indonesia · Português (Brasil)

## Contribuir

Las contribuciones son bienvenidas. Por favor, lee la [Guía de contribución](/CONTRIBUTING.md) antes de abrir una incidencia (issue) o una solicitud de incorporación (pull request).

## Un Agradecimiento Especial a

- [alireza0](https://github.com/alireza0/)

## Reconocimientos

- [Iran v2ray rules](https://github.com/chocolate4u/Iran-v2ray-rules) (Licencia: **GPL-3.0**): _Reglas de enrutamiento mejoradas para v2ray/xray y v2ray/xray-clients con dominios iraníes incorporados y un enfoque en seguridad y bloqueo de anuncios._
- [Russia v2ray rules](https://github.com/runetfreedom/russia-v2ray-rules-dat) (Licencia: **GPL-3.0**): _Este repositorio contiene reglas de enrutamiento V2Ray actualizadas automáticamente basadas en datos de dominios y direcciones bloqueadas en Rusia._

## Herramientas de la Comunidad

Herramientas e integraciones construidas por la comunidad alrededor de 3x-ui.

- [terraform-provider-3x-ui](https://github.com/batonogov/terraform-provider-threexui) (Licencia: **MIT**): _Gestiona inbounds, clientes, configuración del panel y configuración de Xray como código con Terraform / OpenTofu._

## Apoyar el Proyecto

**Si este proyecto te es útil, puedes darle una**:star2:

<a href="https://www.buymeacoffee.com/MHSanaei" target="_blank">
<img src="./media/default-yellow.png" alt="Buy Me A Coffee" style="height: 70px !important;width: 277px !important;" >
</a>

</br>
<a href="https://nowpayments.io/donation/hsanaei" target="_blank" rel="noreferrer noopener">
   <img src="./media/donation-button-black.svg" alt="Crypto donation button by NOWPayments">
</a>

## Estrellas a lo Largo del Tiempo

[![Stargazers over time](https://starchart.cc/MHSanaei/3x-ui.svg?variant=adaptive)](https://starchart.cc/MHSanaei/3x-ui)
