[English](/README.md) | [فارسی](/README.fa_IR.md) | [العربية](/README.ar_EG.md) | [中文](/README.zh_CN.md) | [Español](/README.es_ES.md) | [Русский](/README.ru_RU.md) | [Türkçe](/README.tr_TR.md)

<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="./media/3x-ui-dark.png">
    <img alt="3x-ui" src="./media/3x-ui-light.png">
  </picture>
</p>

<p align="center">
  <a href="https://github.com/Kuzz007/3x-ui/actions"><img src="https://img.shields.io/github/actions/workflow/status/Kuzz007/3x-ui/release.yml.svg" alt="Build"></a>
  <a href="#"><img src="https://img.shields.io/github/go-mod/go-version/Kuzz007/3x-ui.svg" alt="GO Version"></a>
  <a href="https://www.gnu.org/licenses/gpl-3.0.en.html"><img src="https://img.shields.io/badge/license-GPL%20V3-blue.svg?longCache=true" alt="License"></a>
</p>

**Este es un fork personal de [3X-UI](https://github.com/MHSanaei/3x-ui)** — un panel de control web avanzado y de código abierto para [Xray-core](https://github.com/XTLS/Xray-core) — con una incorporación principal: **soporte nativo para AmneziaWG**, como protocolo de primera clase junto a VLESS, VMess, Trojan y el resto. Todo lo que 3X-UI ya hacía (entradas multiprotocolo, contabilidad de tráfico por cliente, suscripciones, multinodo, bot de Telegram) permanece sin cambios y funciona exactamente igual que en el proyecto original.

Este fork se construyó para funcionar en los routers y servidores personales de su autor; no pretende sustituir ni competir con el proyecto original. Si buscas el panel de propósito general, dirígete a [MHSanaei/3x-ui](https://github.com/MHSanaei/3x-ui) — todo lo que sigue documenta únicamente las diferencias de este fork.

> [!IMPORTANT]
> Este proyecto está destinado únicamente al uso personal. Por favor, no lo uses para fines ilegales ni en un entorno de producción.

## En qué se diferencia este fork: AmneziaWG

[AmneziaWG](https://github.com/amnezia-vpn/amneziawg-linux-kernel-module) es una variante de WireGuard que añade una capa de ofuscación (paquetes basura, relleno aleatorio, reescritura de cabeceras mágicas) diseñada para vencer la identificación de huella digital de protocolo basada en DPI — el mismo túnel, pero uno que ya no parece un túnel en el cable.

- **Nativo, no Docker.** AmneziaWG se ejecuta como una interfaz de kernel real en el host, activada y desactivada mediante `awg-quick`/`awg` — el mismo enfoque de módulo de kernel DKMS que te da una interfaz `wg0` nativa. No se necesita ningún contenedor sidecar con privilegios.
- **Un protocolo de primera clase.** Una entrada AmneziaWG vive en la misma tabla `Inbound` que el resto, por lo que obtiene gratuitamente las operaciones masivas, el modal de código QR/descarga de configuración y los enlaces de suscripción — nada nuevo que aprender.
- **Ofuscación completa de AmneziaWG 2.0** — Jc/Jmin/Jmax (paquetes basura), S1–S4 (relleno de paquetes), H1–H4 (cabeceras mágicas) y el paquete de firma I1, todos editables por entrada con un botón de aleatorización de un solo clic, además de un modo compatible con 1.x para clientes más antiguos.
- **IPv6 nativo**, con proxy NDP por cliente para que cada par obtenga una dirección IPv6 directamente accesible — sin NAT66.
- **Reenvío de puertos por cliente** — DNAT de puertos/rangos específicos directamente a la dirección de túnel de un par.
- **Enrutamiento del tráfico de un cliente a través de Xray** — cada entrada AmneziaWG obtiene automáticamente su propio puente Xray por loopback (sin ningún interruptor); enruta el tráfico de cualquier cliente hacia cualquier salida de Xray configurada desde la página de "Enrutamiento" ya existente en el panel, exactamente igual que al enrutar cualquier otro protocolo.
- **`install.sh` instala el módulo de kernel por ti** en Ubuntu/Debian/Armbian (`ppa:amnezia/ppa`), con una alternativa de respaldo para otras distribuciones. Lo único que no puede hacer por ti: **deshabilitar Secure Boot** en tu VPS/VM de antemano — un módulo compilado con DKMS no está firmado, y el kernel se negará a cargarlo mientras Secure Boot esté habilitado.
- La reconciliación se realiza exactamente igual que como [`internal/mtproto`](internal/mtproto) gestiona el sidecar `mtg`: un trabajo en segundo plano mantiene la interfaz en ejecución sincronizada con lo almacenado en la base de datos, aplicando los cambios de pares mediante `awg syncconf` en lugar de un reinicio completo de la interfaz siempre que sea posible.
- **Enlaces de compartición `vpn://` reales** — el enlace de copia/código QR por cliente y el endpoint de suscripción ahora emiten el esquema `vpn://` real que espera la aplicación oficial AmneziaVPN (base64url de un `.conf` plano), no un formato de URI inventado que la app no podía importar.

## Otros cambios en este fork

Las mejoras más pequeñas específicas de este fork, más allá de AmneziaWG, se añaden aquí a medida que se incorporan:

- **Autocompletado de reglas de enrutamiento** — los campos Domain/IP del editor de reglas de Routing de Xray ahora sugieren categorías geosite/geoip (por ejemplo, escribir "you" sugiere `geosite:youtube`) generadas en vivo a partir de los archivos `.dat` realmente instalados en la carpeta bin de Xray, incluidos los archivos personalizados añadidos mediante la función de actualización automática de Geodata (por ejemplo, `geosite_roscom.dat`). La entrada de texto libre sigue funcionando exactamente igual que antes.
- **Velocidad en vivo para AmneziaWG y MTProto** — la columna Speed mostraba «--» para las entradas/clientes de AmneziaWG y MTProto (`mtg`) aunque los totales de tráfico acumulado eran correctos, ya que ninguno de los dos se ejecuta dentro del propio runtime de Xray-core y por tanto son invisibles para su API de estadísticas. Ahora ambos difunden la velocidad en vivo junto con el resto de protocolos.
- **Los archivos personalizados en `bin/` sobreviven a las actualizaciones** — una reinstalación/actualización solía borrar toda la carpeta `bin/` antes de volver a extraer la versión, eliminando silenciosamente cualquier cosa colocada allí manualmente (lo más común, un archivo geoip/geosite personalizado referenciado desde una regla de enrutamiento mediante `ext:<file>:<code>`) y rompiendo todas las entradas en el siguiente inicio. El instalador ahora respalda `bin/` primero y restaura solo lo que la nueva versión no incluye.

## Características

- **Entradas multiprotocolo** — VLESS, VMess, Trojan, Shadowsocks, WireGuard, **AmneziaWG**, Hysteria2, HTTP, SOCKS (Mixed), Dokodemo-door / Tunnel y TUN.
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
curl -fsSL https://raw.githubusercontent.com/Kuzz007/3x-ui/main/install.sh | bash
```

Para instalar una versión específica, añade su etiqueta (p. ej. `v3.5.0-awg.1`):

```bash
curl -fsSL https://raw.githubusercontent.com/Kuzz007/3x-ui/main/install.sh | bash -s v3.5.0-awg.1
```

Para instalar la compilación continua **dev** (el último prelanzamiento por commit desde `main`, no una versión estable), pasa `dev`:

```bash
curl -fsSL https://raw.githubusercontent.com/Kuzz007/3x-ui/main/install.sh | bash -s dev
```

Los lanzamientos estables propios de este fork se etiquetan como `<versión base de upstream>-awg.N` (p. ej. `v3.5.0-awg.1`, construido sobre lo que upstream llama `v3.5.0`) — nunca un simple `vX.Y.Z` — para que nunca se confundan con un lanzamiento real de MHSanaei/3x-ui con el mismo número.

Durante la instalación se generan un nombre de usuario, una contraseña y una ruta de acceso aleatorios. Tras la instalación, ejecuta `x-ui` para abrir el menú de gestión, donde puedes iniciar/detener el servicio, ver o restablecer tus credenciales de acceso, gestionar certificados SSL y mucho más.

Para la documentación completa del panel más allá de lo que cubre este README, visita la [Wiki del proyecto original](https://github.com/MHSanaei/3x-ui/wiki) — nada de ella es específico de este fork, así que sigue siendo totalmente válida.

### Instalación desatendida

El instalador también se ejecuta de forma **no interactiva** para cloud-init.
Define `XUI_NONINTERACTIVE=1` (o canalízalo sin TTY) y realizará la instalación de principio a fin sin
ninguna pregunta, generando credenciales aleatorias y escribiéndolas en
`/etc/x-ui/install-result.env`. Consulta [`deploy/`](deploy/) para:

- [User-data de cloud-init](deploy/cloud-init/) — instalación desatendida en cualquier nube (Hetzner/AWS/DO/Vultr/GCP/Azure/Oracle)
- [Notas de Hetzner Cloud](deploy/marketplace/hetzner/) — despliegue basado en cloud-init en Hetzner

## Plataformas Compatibles

**Sistemas operativos:** Ubuntu, Debian, Armbian, Fedora, CentOS, RHEL, AlmaLinux, Rocky Linux, Oracle Linux, Amazon Linux, Virtuozzo, Arch, Manjaro, Parch, openSUSE (Tumbleweed / Leap) y Alpine. (El proyecto original también publica una versión para Windows; la CI de este fork no lo hace — todo aquí está orientado a servidores/routers que ejecutan Linux, y AmneziaWG necesita un módulo de kernel de Linux de todos modos.)

**Arquitecturas:** `amd64` · `386` · `arm64` (aarch64) · `armv7` · `armv6` · `armv5` · `s390x`.

AmneziaWG requiere específicamente un kernel de Linux real con el módulo de kernel DKMS de AmneziaWG — no funcionará en Windows, y `install_amneziawg` hoy solo automatiza la instalación del módulo de kernel en Ubuntu/Debian/Armbian (consulta la sección [En qué se diferencia este fork](#en-qué-se-diferencia-este-fork-amneziawg)).

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

## Notas para Desarrolladores

Este es un fork personal y no busca colaboradores externos, pero [CONTRIBUTING.md](/CONTRIBUTING.md) sigue conteniendo instrucciones detalladas y útiles para configurar un entorno de desarrollo local (versiones de Go/Node, el compilador de C requerido para CGo, comandos de build/lint/test), por si tú mismo trabajas sobre esta base de código.

## Reconocimiento

Este fork está construido enteramente sobre [MHSanaei/3x-ui](https://github.com/MHSanaei/3x-ui) — todo el panel, el soporte multiprotocolo y la arquitectura subyacente son obra suya; **el soporte de AmneziaWG es lo único añadido aquí.** Si el proyecto original te resulta útil, los enlaces de apoyo del autor original siguen siendo el lugar correcto para ello:

<a href="https://www.buymeacoffee.com/MHSanaei" target="_blank">
<img src="./media/default-yellow.png" alt="Buy Me A Coffee" style="height: 70px !important;width: 277px !important;" >
</a>

</br>
<a href="https://nowpayments.io/donation/hsanaei" target="_blank" rel="noreferrer noopener">
   <img src="./media/donation-button-black.svg" alt="Crypto donation button by NOWPayments">
</a>

La implementación nativa de AmneziaWG en este fork se basa en / está inspirada por:

- [MHSanaei/3x-ui#6086](https://github.com/MHSanaei/3x-ui/pull/6086) — el pull request original de AmneziaWG contra el proyecto original (enfoque de sidecar Docker); este fork reutiliza su estructura de schema/frontend pero reemplaza el backend por un gestor nativo sin Docker.
- [coinman-dev/3ax-ui](https://github.com/coinman-dev/3ax-ui) — un fork independiente que ya ejecuta AmneziaWG nativo en producción; la gestión del proceso `awg-quick`, la generación de configuración y el generador de parámetros de ofuscación de AmneziaWG 2.0 de este fork provienen de su paquete `awg/`.

## Un Agradecimiento Especial a

- [alireza0](https://github.com/alireza0/)
- [Iran v2ray rules](https://github.com/chocolate4u/Iran-v2ray-rules) (Licencia: **GPL-3.0**): _Reglas de enrutamiento mejoradas para v2ray/xray y v2ray/xray-clients con dominios iraníes incorporados y un enfoque en seguridad y bloqueo de anuncios._
- [Russia v2ray rules](https://github.com/runetfreedom/russia-v2ray-rules-dat) (Licencia: **GPL-3.0**): _Este repositorio contiene reglas de enrutamiento V2Ray actualizadas automáticamente basadas en datos de dominios y direcciones bloqueadas en Rusia._

## Herramientas de la Comunidad

Herramientas e integraciones construidas por la comunidad alrededor de 3x-ui.

- [terraform-provider-3x-ui](https://github.com/batonogov/terraform-provider-threexui) (Licencia: **MIT**): _Gestiona inbounds, clientes, configuración del panel y configuración de Xray como código con Terraform / OpenTofu._
