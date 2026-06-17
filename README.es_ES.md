[English](/README.md) | [فارسی](/README.fa_IR.md) | [العربية](/README.ar_EG.md) | [中文](/README.zh_CN.md) | [Español](/README.es_ES.md) | [Русский](/README.ru_RU.md) | [Türkçe](/README.tr_TR.md)

<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="./media/dune-dark.png">
    <img alt="dune" src="./media/dune-light.png">
  </picture>
</p>

<p align="center">
  <a href="https://github.com/leto217/DUNE/releases"><img src="https://img.shields.io/github/v/release/leto217/DUNE" alt="Release"></a>
  <a href="https://github.com/leto217/DUNE/actions"><img src="https://img.shields.io/github/actions/workflow/status/leto217/DUNE/release.yml.svg" alt="Build"></a>
  <a href="#"><img src="https://img.shields.io/github/go-mod/go-version/leto217/DUNE.svg" alt="GO Version"></a>
  <a href="https://github.com/leto217/DUNE/releases/latest"><img src="https://img.shields.io/github/downloads/leto217/DUNE/total.svg" alt="Downloads"></a>
  <a href="https://www.gnu.org/licenses/gpl-3.0.en.html"><img src="https://img.shields.io/badge/license-GPL%20V3-blue.svg?longCache=true" alt="License"></a>
  <a href="https://pkg.go.dev/github.com/leto217/DUNE"><img src="https://pkg.go.dev/badge/github.com/leto217/DUNE.svg" alt="Go Reference"></a>
  <a href="https://goreportcard.com/report/github.com/leto217/DUNE"><img src="https://goreportcard.com/badge/github.com/leto217/DUNE" alt="Go Report Card"></a>
</p>

**DUNE** es un fork ligero de [3X-UI](https://github.com/MHSanaei/3x-ui): un panel de control web de código abierto para gestionar servidores [Xray-core](https://github.com/XTLS/Xray-core). Conserva los flujos de trabajo y la cobertura de protocolos de 3X-UI, pero consume mucho menos CPU y RAM, ideal para VPS pequeños y entornos con pocos recursos.

Derivado de 3X-UI con foco en eficiencia, DUNE reduce tareas en segundo plano, optimiza el uso de memoria y simplifica la pila para que el panel siga siendo ágil sin saturar el servidor.

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
- **Servidor de suscripción integrado** con múltiples formatos de salida.
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
bash <(curl -Ls https://raw.githubusercontent.com/leto217/DUNE/main/install.sh)
```

Durante la instalación se generan un nombre de usuario, una contraseña y una ruta de acceso aleatorios. Tras la instalación, ejecuta `dune` para abrir el menú de gestión, donde puedes iniciar/detener el servicio, ver o restablecer tus credenciales de acceso, gestionar certificados SSL y mucho más.

Para la documentación completa, visita la [Wiki del proyecto](https://github.com/leto217/DUNE/wiki).

## Plataformas Compatibles

**Sistemas operativos:** Ubuntu, Debian, Armbian, Fedora, CentOS, RHEL, AlmaLinux, Rocky Linux, Oracle Linux, Amazon Linux, Virtuozzo, Arch, Manjaro, Parch, openSUSE (Tumbleweed / Leap), Alpine y Windows.

**Arquitecturas:** `amd64` · `386` · `arm64` (aarch64) · `armv7` · `armv6` · `armv5` · `s390x`.

## Opciones de Base de Datos

Dune admite dos backends, que se eligen durante la instalación:

- **SQLite** (predeterminado) — un único archivo en `/etc/dune/dune.db`. Sin configuración, ideal para despliegues pequeños y medianos.
- **PostgreSQL** — recomendado para un gran número de clientes o configuraciones multinodo. El instalador puede instalar PostgreSQL localmente por ti, o aceptar un DSN a un servidor existente.

En tiempo de ejecución, el backend se selecciona mediante variables de entorno (el instalador las escribe por ti en `/etc/default/dune`):

```
DUNE_DB_TYPE=postgres
DUNE_DB_DSN=postgres://dune:password@127.0.0.1:5432/dune?sslmode=disable
```

### Migrar una instalación de SQLite existente a PostgreSQL

```bash
dune migrate-db --dsn "postgres://dune:password@127.0.0.1:5432/dune?sslmode=disable"
# luego define DUNE_DB_TYPE y DUNE_DB_DSN en /etc/default/dune y reinicia:
systemctl restart dune
```

El archivo SQLite de origen permanece intacto; elimínalo manualmente una vez que hayas verificado el nuevo backend.

### Docker

El comando predeterminado `docker compose up -d` sigue usando SQLite. Para ejecutarlo con el servicio PostgreSQL incluido, descomenta las dos líneas de variables de entorno `DUNE_DB_*` en `docker-compose.yml` e inícialo con el perfil:

```bash
docker compose --profile postgres up -d
```

La imagen incluye Fail2ban (habilitado de forma predeterminada) para aplicar **límites de IP** por cliente. Fail2ban banea a los infractores con `iptables`, lo que requiere la capacidad `NET_ADMIN`. `docker-compose.yml` ya la concede mediante `cap_add`; si en su lugar inicias el contenedor con `docker run`, añade tú mismo las capacidades, de lo contrario los baneos se registran pero nunca se aplican:

```bash
docker run -d --cap-add=NET_ADMIN --cap-add=NET_RAW ... ghcr.io/leto217/DUNE
```

## Variables de Entorno

| Variable | Descripción | Predeterminado |
| --- | --- | --- |
| `DUNE_DB_TYPE` | Backend de base de datos: `sqlite` o `postgres` | `sqlite` |
| `DUNE_DB_DSN` | Cadena de conexión de PostgreSQL (cuando `DUNE_DB_TYPE=postgres`) | — |
| `DUNE_DB_FOLDER` | Directorio del archivo de base de datos SQLite | `/etc/dune` |
| `DUNE_DB_MAX_OPEN_CONNS` | Máximo de conexiones abiertas (pool de PostgreSQL) | — |
| `DUNE_DB_MAX_IDLE_CONNS` | Máximo de conexiones inactivas (pool de PostgreSQL) | — |
| `DUNE_INIT_WEB_BASE_PATH` | La ruta URI inicial para el panel web | `/` |
| `DUNE_ENABLE_FAIL2BAN` | Habilitar la aplicación de límites de IP basada en Fail2ban | `true` |
| `DUNE_LOG_LEVEL` | Nivel de registro (`debug`, `info`, `warning`, `error`) | `info` |
| `DUNE_DEBUG` | Habilitar el modo de depuración | `false` |

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

Herramientas e integraciones construidas por la comunidad alrededor de dune.

- [terraform-provider-dune](https://github.com/batonogov/terraform-provider-threexui) (Licencia: **MIT**): _Gestiona inbounds, clientes, configuración del panel y configuración de Xray como código con Terraform / OpenTofu._

## Apoyar el Proyecto

**Si este proyecto te es útil, puedes darle una**:star2:

| Red | Dirección |
| --- | --- |
| TON | `UQAa5FpNlK8Gp7tO8luJXHD-Sf0pPjJbNHGo8hdkyuUBhWEa` |
| TRON | `TLqtTfYSzPLFm8mtFDkSnXvzucxx7DS5VL` |
| ERC20 and BEP20 | `0x2fe632d70f4612b87670f8a28b4587ea2641452d` |

## Estrellas a lo Largo del Tiempo

[![Stargazers over time](https://starchart.cc/leto217/DUNE.svg?variant=adaptive)](https://starchart.cc/leto217/DUNE)
