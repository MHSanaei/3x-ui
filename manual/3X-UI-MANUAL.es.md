# Manual de usuario del panel 3X-UI

🇸🇦 [العربية](3X-UI-MANUAL.ar.md) · 🇬🇧 [English](3X-UI-MANUAL.en.md) · 🇪🇸 Español · 🇮🇷 [فارسی](3X-UI-MANUAL.fa.md) · 🇮🇩 [Bahasa Indonesia](3X-UI-MANUAL.id.md) · 🇯🇵 [日本語](3X-UI-MANUAL.ja.md) · 🇧🇷 [Português](3X-UI-MANUAL.pt.md) · 🇷🇺 [Русский](3X-UI-MANUAL.ru.md) · 🇹🇷 [Türkçe](3X-UI-MANUAL.tr.md) · 🇺🇦 [Українська](3X-UI-MANUAL.uk.md) · 🇻🇳 [Tiếng Việt](3X-UI-MANUAL.vi.md) · 🇨🇳 [简体中文](3X-UI-MANUAL.zh-CN.md) · 🇹🇼 [繁體中文](3X-UI-MANUAL.zh-TW.md)

**Versión de 3X-UI: 3.4.1.** Este manual ha sido elaborado para esta versión y es válido para ella. El resumen de cambios de 3.4.1 respecto a 3.4.0 se encuentra en la sección [«Qué hay de nuevo en 3.4.1»](#qué-hay-de-nuevo-en-341).

> Manual detallado en español sobre el panel web **3X-UI** (gestión de
> Xray-core): funciones, configuración y operación, con descripción de cada
> campo e interruptor de la interfaz.
>
> Los nombres y etiquetas corresponden a la interfaz del panel. Las palabras *inbound* / *outbound* no
> se traducen.

## Contenido

- [Qué hay de nuevo en 3.4.1](#qué-hay-de-nuevo-en-341)
- [1. Introducción, requisitos e instalación](#1-introducción-requisitos-e-instalación)
  - [1.1. Qué es 3X-UI](#11-qué-es-3x-ui)
  - [1.2. Sistemas operativos y arquitecturas compatibles](#12-sistemas-operativos-y-arquitecturas-compatibles)
  - [1.3. Métodos de instalación](#13-métodos-de-instalación)
  - [1.4. Primer inicio y credenciales por defecto](#14-primer-inicio-y-credenciales-por-defecto)
  - [1.5. Ubicación de archivos](#15-ubicación-de-archivos)
  - [1.6. Comando de gestión `x-ui` (menú del script)](#16-comando-de-gestión-x-ui-menú-del-script)
  - [1.7. Subcomandos de `x-ui` (sin menú interactivo)](#17-subcomandos-de-x-ui-sin-menú-interactivo)
  - [1.8. Migración SQLite → PostgreSQL](#18-migración-sqlite--postgresql)
- [2. Acceso al panel y seguridad](#2-acceso-al-panel-y-seguridad)
  - [2.1. Formulario de inicio de sesión](#21-formulario-de-inicio-de-sesión)
  - [2.2. Autenticación de doble factor (2FA / TOTP)](#22-autenticación-de-doble-factor-2fa--totp)
  - [2.3. Limitación de intentos de inicio de sesión (login limiter / protección contra fuerza bruta)](#23-limitación-de-intentos-de-inicio-de-sesión-login-limiter--protección-contra-fuerza-bruta)
  - [2.4. Cambio de nombre de usuario y contraseña del administrador](#24-cambio-de-nombre-de-usuario-y-contraseña-del-administrador)
  - [2.5. Ruta secreta (ruta URI / webBasePath) y puerto del panel](#25-ruta-secreta-ruta-uri--webbasepath-y-puerto-del-panel)
  - [2.6. Tiempo de vida de la sesión (timeout)](#26-tiempo-de-vida-de-la-sesión-timeout)
  - [2.7. LDAP (sincronización y autenticación)](#27-ldap-sincronización-y-autenticación)
- [3. Resumen / Dashboard](#3-resumen--dashboard)
  - [3.1. Principios generales de recolección de datos](#31-principios-generales-de-recolección-de-datos)
  - [3.2. CPU](#32-cpu)
  - [3.3. Memoria (RAM)](#33-memoria-ram)
  - [3.4. Swap](#34-swap)
  - [3.5. Disco (Storage)](#35-disco-storage)
  - [3.6. Tiempo de actividad del sistema (Uptime)](#36-tiempo-de-actividad-del-sistema-uptime)
  - [3.7. Carga del sistema (Load average)](#37-carga-del-sistema-load-average)
  - [3.8. Red: velocidad y volumen total de tráfico](#38-red-velocidad-y-volumen-total-de-tráfico)
  - [3.9. Direcciones IP del servidor](#39-direcciones-ip-del-servidor)
  - [3.10. Conexiones TCP/UDP](#310-conexiones-tcpudp)
  - [3.11. Estado de Xray y control del proceso](#311-estado-de-xray-y-control-del-proceso)
  - [3.12. Actualización del panel (3X-UI)](#312-actualización-del-panel-3x-ui)
  - [3.13. Actualización de archivos geográficos (GeoIP / GeoSite)](#313-actualización-de-archivos-geográficos-geoip--geosite)
  - [3.14. Copia de seguridad y restauración de la base de datos](#314-copia-de-seguridad-y-restauración-de-la-base-de-datos)
  - [3.15. Elementos adicionales de la interfaz](#315-elementos-adicionales-de-la-interfaz)
- [4. Inbounds: creación y parámetros generales](#4-inbounds-creación-y-parámetros-generales)
  - [4.1. Campos generales del formulario](#41-campos-generales-del-formulario)
  - [4.2. Sniffing (Análisis de tráfico)](#42-sniffing-análisis-de-tráfico)
  - [4.3. Allocate (estrategia de distribución de puertos)](#43-allocate-estrategia-de-distribución-de-puertos)
  - [4.4. External Proxy (Proxy externo)](#44-external-proxy-proxy-externo)
  - [4.5. Fallbacks (Fallbacks)](#45-fallbacks-fallbacks)
  - [4.6. Reinicio periódico del tráfico](#46-reinicio-periódico-del-tráfico)
  - [4.7. JSON del entrante (avanzado)](#47-json-del-entrante-avanzado)
  - [4.8. Acciones sobre el inbound: QR / Edit / Reset / Delete y estadísticas](#48-acciones-sobre-el-inbound-qr--edit--reset--delete-y-estadísticas)
- [5. Protocolos](#5-protocolos)
  - [5.1. Lista de protocolos admitidos](#51-lista-de-protocolos-admitidos)
  - [5.2. Qué protocolos admiten TLS / REALITY / transporte](#52-qué-protocolos-admiten-tls--reality--transporte)
  - [5.3. VLESS](#53-vless)
  - [5.4. VMess](#54-vmess)
  - [5.5. Trojan](#55-trojan)
  - [5.6. Shadowsocks](#56-shadowsocks)
  - [5.7. Dokodemo-door / Tunnel (reenviador transparente)](#57-dokodemo-door--tunnel-reenviador-transparente)
  - [5.8. SOCKS / HTTP (protocolo `mixed`)](#58-socks--http-protocolo-mixed)
  - [5.9. WireGuard (inbound)](#59-wireguard-inbound)
  - [5.10. Hysteria (v2 por defecto)](#510-hysteria-v2-por-defecto)
  - [5.11. MTProto (proxy para Telegram)](#511-mtproto-proxy-para-telegram)
  - [5.12. Guía rápida para elegir protocolo](#512-guía-rápida-para-elegir-protocolo)
- [6. Transporte (Stream Settings)](#6-transporte-stream-settings)
  - [6.1. Selección de la red de transmisión](#61-selección-de-la-red-de-transmisión)
  - [6.2. RAW / TCP (`tcpSettings`)](#62-raw--tcp-tcpsettings)
  - [6.3. mKCP (`kcpSettings`)](#63-mkcp-kcpsettings)
  - [6.4. WebSocket (`wsSettings`)](#64-websocket-wssettings)
  - [6.5. gRPC (`grpcSettings`)](#65-grpc-grpcsettings)
  - [6.6. HTTPUpgrade (`httpupgradeSettings`)](#66-httpupgrade-httpupgradesettings)
  - [6.7. XHTTP / SplitHTTP (`xhttpSettings`)](#67-xhttp--splithttp-xhttpsettings)
  - [6.8. Transporte Hysteria (`hysteriaSettings`)](#68-transporte-hysteria-hysteriasettings)
  - [6.9. Parámetros complementarios](#69-parámetros-complementarios)
- [7. Seguridad de la conexión: TLS, XTLS y REALITY](#7-seguridad-de-la-conexión-tls-xtls-y-reality)
  - [7.1. Diferencias: TLS vs XTLS vs REALITY](#71-diferencias-tls-vs-xtls-vs-reality)
  - [7.2. Modo «Ninguno» (`none`)](#72-modo-ninguno-none)
  - [7.3. Modo TLS](#73-modo-tls)
  - [7.4. Modo REALITY](#74-modo-reality)
  - [7.5. Recomendaciones prácticas de configuración](#75-recomendaciones-prácticas-de-configuración)
- [8. Clientes](#8-clientes)
  - [8.1. Campos del cliente](#81-campos-del-cliente)
  - [8.2. Vinculación al inbound](#82-vinculación-al-inbound)
  - [8.3. Operaciones sobre el cliente](#83-operaciones-sobre-el-cliente)
  - [8.4. Operaciones masivas](#84-operaciones-masivas)
  - [8.5. Búsqueda, filtros y ordenación](#85-búsqueda-filtros-y-ordenación)
  - [8.6. Iconos y estados](#86-iconos-y-estados)
- [9. Grupos de clientes](#9-grupos-de-clientes)
  - [9.1. Qué es un grupo de clientes y para qué sirve](#91-qué-es-un-grupo-de-clientes-y-para-qué-sirve)
  - [9.2. Relación del grupo con clientes, inbound, nodos y protocolos](#92-relación-del-grupo-con-clientes-inbound-nodos-y-protocolos)
  - [9.3. Catálogo de grupos y grupos «vacíos»](#93-catálogo-de-grupos-y-grupos-vacíos)
  - [9.4. Campos y columnas del grupo](#94-campos-y-columnas-del-grupo)
  - [9.5. Creación de un grupo](#95-creación-de-un-grupo)
  - [9.6. Cambio de nombre de un grupo](#96-cambio-de-nombre-de-un-grupo)
  - [9.7. Añadir clientes a un grupo](#97-añadir-clientes-a-un-grupo)
  - [9.8. Eliminación de clientes de un grupo (sin eliminar los propios clientes)](#98-eliminación-de-clientes-de-un-grupo-sin-eliminar-los-propios-clientes)
  - [9.9. Reinicio del tráfico del grupo](#99-reinicio-del-tráfico-del-grupo)
  - [9.10. Eliminación del grupo y eliminación de clientes del grupo](#910-eliminación-del-grupo-y-eliminación-de-clientes-del-grupo)
  - [9.11. Relación con la página «Clientes»](#911-relación-con-la-página-clientes)
  - [9.12. Resumen de endpoints de API](#912-resumen-de-endpoints-de-api)
  - [9.13. Tráfico por grupo](#913-tráfico-por-grupo)
- [10. Suscripciones (Subscription)](#10-suscripciones-subscription)
  - [10.1. Qué es subId y cómo se forma el enlace](#101-qué-es-subid-y-cómo-se-forma-el-enlace)
  - [10.2. Configuración del servidor de suscripciones](#102-configuración-del-servidor-de-suscripciones)
  - [10.3. Formatos de salida](#103-formatos-de-salida)
  - [10.4. Página de información de la suscripción y códigos QR](#104-página-de-información-de-la-suscripción-y-códigos-qr)
  - [10.5. Plantillas personalizadas de la página de suscripción](#105-plantillas-personalizadas-de-la-página-de-suscripción)
- [11. Xray: enrutamiento, outbounds, DNS y extensiones](#11-xray-enrutamiento-outbounds-dns-y-extensiones)
  - [11.1. Estructura del editor: pestañas/modos](#111-estructura-del-editor-pestañasmodos)
  - [11.2. Configuración principal (General)](#112-configuración-principal-general)
  - [11.3. Reglas de enrutamiento (routing)](#113-reglas-de-enrutamiento-routing)
  - [11.4. Outbounds (conexiones salientes)](#114-outbounds-conexiones-salientes)
  - [11.5. Balanceadores (Balancers)](#115-balanceadores-balancers)
  - [11.6. DNS](#116-dns)
  - [11.7. Fake DNS](#117-fake-dns)
  - [11.8. WireGuard / WARP / NordVPN](#118-wireguard--warp--nordvpn)
  - [11.9. Reverse-proxy y TUN](#119-reverse-proxy-y-tun)
  - [11.10. Logs y estadísticas (Stats, metrics)](#1110-logs-y-estadísticas-stats-metrics)
  - [11.11. Guardado, reinicio y transformaciones automáticas](#1111-guardado-reinicio-y-transformaciones-automáticas)
  - [11.12. Outbound de suscripción (con actualización automática)](#1112-outbound-de-suscripción-con-actualización-automática)
  - [11.13. Rotación de IP en WARP](#1113-rotación-de-ip-en-warp)
- [12. Nodos (multipanel, master/slave)](#12-nodos-multipanel-masterslave)
  - [12.1. Resumen en la parte superior de la lista](#121-resumen-en-la-parte-superior-de-la-lista)
  - [12.2. Añadir y editar un nodo](#122-añadir-y-editar-un-nodo)
  - [12.3. Verificación TLS (para nodos https)](#123-verificación-tls-para-nodos-https)
  - [12.4. Qué se muestra por cada nodo](#124-qué-se-muestra-por-cada-nodo)
  - [12.5. Acciones sobre el nodo](#125-acciones-sobre-el-nodo)
  - [12.6. Historial de métricas](#126-historial-de-métricas)
  - [12.7. Cómo se sincronizan los inbounds y los clientes](#127-cómo-se-sincronizan-los-inbounds-y-los-clientes)
  - [12.8. Cadenas de nodos (subnodos / nodos transitivos)](#128-cadenas-de-nodos-subnodos--nodos-transitivos)
  - [12.9. Nodos: novedades en 3.3.0](#129-nodos-novedades-en-330)
- [13. Configuración del panel](#13-configuración-del-panel)
  - [13.1. Guardar y reiniciar el panel](#131-guardar-y-reiniciar-el-panel)
  - [13.2. Configuración general (pestaña «Panel» / *General*)](#132-configuración-general-pestaña-panel--general)
  - [13.3. Acceso al panel: IP, puerto, ruta, dominio, certificado](#133-acceso-al-panel-ip-puerto-ruta-dominio-certificado)
  - [13.4. Sesión, proxy del panel y proxies de confianza (pestaña «Proxy y servidor» / *Proxy and Server*)](#134-sesión-proxy-del-panel-y-proxies-de-confianza-pestaña-proxy-y-servidor--proxy-and-server)
  - [13.5. Bot de Telegram (pestaña «Bot de Telegram» / *Telegram Bot*)](#135-bot-de-telegram-pestaña-bot-de-telegram--telegram-bot)
  - [13.6. Fecha y hora (pestaña «Fecha y hora» / *Date and Time*)](#136-fecha-y-hora-pestaña-fecha-y-hora--date-and-time)
  - [13.7. Tráfico externo y comportamiento de Xray (pestaña «Tráfico externo» / *External Traffic*)](#137-tráfico-externo-y-comportamiento-de-xray-pestaña-tráfico-externo--external-traffic)
  - [13.8. Otros: plantilla de configuración de Xray y URL de verificación](#138-otros-plantilla-de-configuración-de-xray-y-url-de-verificación)
  - [13.9. Cuenta de administrador y tokens de API](#139-cuenta-de-administrador-y-tokens-de-api)
  - [13.10. Cambios de API en 3.3.0 (importante para integraciones)](#1310-cambios-de-api-en-330-importante-para-integraciones)
- [14. Bot de Telegram](#14-bot-de-telegram)
  - [14.1. Activación y configuración del bot](#141-activación-y-configuración-del-bot)
  - [14.2. Menú principal y botones](#142-menú-principal-y-botones)
  - [14.3. Comandos del bot](#143-comandos-del-bot)
  - [14.4. Gestión de clientes (solo administrador)](#144-gestión-de-clientes-solo-administrador)
  - [14.5. Notificaciones e informes](#145-notificaciones-e-informes)
  - [14.6. Copia de seguridad y registros](#146-copia-de-seguridad-y-registros)
  - [14.7. Particularidades de funcionamiento](#147-particularidades-de-funcionamiento)
- [15. Bases geográficas (geoip / geosite y personalizadas)](#15-bases-geográficas-geoip--geosite-y-personalizadas)
  - [15.1. Qué son geoip.dat y geosite.dat](#151-qué-son-geoipdat-y-geositedat)
  - [15.2. Archivos geográficos estándar y su actualización](#152-archivos-geográficos-estándar-y-su-actualización)
  - [15.3. Actualización automática de geodatos mediante Xray (Geodata Auto-Update)](#153-actualización-automática-de-geodatos-mediante-xray-geodata-auto-update)
  - [15.4. Validación y restricciones](#154-validación-y-restricciones)
  - [15.5. Verificación automática al arrancar el panel](#155-verificación-automática-al-arrancar-el-panel)
  - [15.6. Uso de las bases geográficas en las reglas de enrutamiento](#156-uso-de-las-bases-geográficas-en-las-reglas-de-enrutamiento)
- [16. Operaciones: copias de seguridad, registros, actualización, CLI](#16-operaciones-copias-de-seguridad-registros-actualización-cli)
  - [16.1. Copia de seguridad y restauración de la base de datos](#161-copia-de-seguridad-y-restauración-de-la-base-de-datos)
  - [16.2. Visualización de registros](#162-visualización-de-registros)
  - [16.3. Nivel y configuración del registro de Xray](#163-nivel-y-configuración-del-registro-de-xray)
  - [16.4. Gestión de Xray: detención y reinicio](#164-gestión-de-xray-detención-y-reinicio)
  - [16.5. Reinicio y actualización del panel](#165-reinicio-y-actualización-del-panel)
  - [16.6. Tareas periódicas (cron)](#166-tareas-periódicas-cron)
  - [16.7. Menú de consola y CLI (`x-ui`)](#167-menú-de-consola-y-cli-x-ui)
  - [16.8. Eliminación del panel](#168-eliminación-del-panel)
  - [16.9. Comando `x-ui migrateDB`](#169-comando-x-ui-migratedb)

## Qué hay de nuevo en 3.4.1

Esta sección enumera brevemente los cambios de la versión **3.4.1** respecto a 3.4.0 visibles para el usuario del panel, agrupados por secciones del manual. Los detalles de cada función se encuentran en la sección correspondiente a continuación.

### Cambios en la sección 1 — Introducción, requisitos e instalación
- **Instalación de la compilación dev e instalación de una versión específica mediante install.sh** — El script de instalación install.sh ahora admite un argumento para seleccionar la versión: especifique una etiqueta (por ejemplo, v3.4.0) para instalar una versión concreta, o 'dev-latest' (alias 'dev') para instalar la compilación rolling dev según el último commit de main, omitiendo la comprobación de versión mínima. Sin argumento se instala el último lanzamiento estable.

### Cambios en la sección 3 — Resumen / Panel de control
- **Panel de control: rediseño del selector de rango en los gráficos de historial del sistema y métricas de Xray** — En las ventanas de historial del panel de control se ha renovado la selección del intervalo de tiempo. Para los gráficos de métricas del sistema están disponibles los rangos 2m, 1h, 3h, 6h, 12h, 24h, 2d y 7d (el historial se almacena hasta 7 días en lugar de las anteriores 48 horas), y en los rangos de 2 y 7 días las etiquetas de tiempo incluyen la fecha. Para los gráficos de métricas de Xray están disponibles los rangos 2m, 1h, 3h, 6h y 12h. Los valores irregulares 30m, 2h y 5h se han eliminado.
- **Panel de control: la tarjeta de uso de memoria muestra el RSS real del proceso** — El indicador de uso de memoria RAM del panel en el panel de control ahora refleja el RSS real del proceso y coincide con el valor que muestra el sistema operativo. Anteriormente se mostraba el contador interno de Go, que sobreestimaba el consumo de memoria y nunca disminuía. Ahora el número baja a medida que se libera memoria.

### Cambios en la sección 5 — Protocolos
- **Cifrado VLESS: nuevos modos de generación de claves (native / xorpub / random)** — En el inbound con protocolo VLESS el bloque de generación de claves de cifrado ha cambiado. En lugar de dos botones separados (X25519 y ML-KEM-768) bajo los campos «Descifrado» y «Cifrado» aparece una lista desplegable «Generación de claves» con seis opciones: X25519 y ML-KEM-768, cada una en tres modos — native, xorpub y random. Seleccione el modo deseado y pulse «Generar»: el panel rellenará los campos decryption y encryption con el par de claves generado. El botón «Limpiar» borra los valores generados y la línea «Seleccionado» muestra el tipo y modo de clave actuales.
- **Limpiar el campo Rewrite port en la configuración del tunnel-inbound ya no rompe el guardado** — Se ha corregido un error: en el inbound con protocolo tunnel, limpiar el campo «Puerto de reescritura» (Rewrite port) ya no provoca un error al guardar. Anteriormente el valor vacío generaba un mensaje de error de validación; ahora el campo simplemente se excluye de la configuración al limpiarlo.

### Cambios en la sección 7 — Seguridad de la conexión: TLS, XTLS y REALITY
- **Restauración del flow XTLS Vision al activar el cifrado en un inbound existente** — Si se activa el cifrado (decryption/encryption) en un inbound VLESS/XHTTP existente después de que ya se hayan añadido clientes, el panel ahora restaura automáticamente flow=xtls-rprx-vision en los clientes que lo requieren. Antes, el flow desaparecía silenciosamente de las configuraciones, enlaces y suscripciones en este caso (especialmente en los inbounds de nodos). No se requiere ninguna acción manual: la corrección se aplica automáticamente al editar el inbound y una vez al actualizar el panel.

### Cambios en la sección 8 — Clientes
- **Activación y desactivación masiva de clientes seleccionados** — Al seleccionar varios clientes en la página Clients, en el menú More (Más) están disponibles las acciones masivas Enable (Activar) y Disable (Desactivar). La activación habilita cada cliente seleccionado en todos los inbounds vinculados; los clientes con cuota de tráfico agotada o plazo vencido se desactivarán automáticamente de nuevo. La desactivación priva inmediatamente a los clientes del acceso, pero sus registros y el tráfico acumulado se conservan. Antes de ejecutar la acción el panel solicita confirmación, y tras la operación muestra una notificación con el número de clientes procesados y, si los hay, con el número de aquellos para los que la acción no tuvo éxito.
- **Configuración masiva de XTLS flow en el diálogo Adjust** — En el diálogo de ajuste masivo Adjust ha aparecido el campo Set flow para establecer o restablecer el XTLS flow en todos los clientes seleccionados a la vez. Por defecto se selecciona No change (sin cambios). La opción Disable (clear flow) restablece el flow, y los valores xtls-rprx-vision y xtls-rprx-vision-udp443 establecen el vision-flow correspondiente. La configuración del vision-flow solo se aplica a los inbounds que admiten flow; los inbounds incompatibles permanecen sin cambios y se marcan como omitidos, mientras que el restablecimiento del flow siempre está permitido. Ahora basta con especificar días, tráfico o flow para aplicar el diálogo.
- **Renombrar un cliente ya no rompe los vínculos y se ha eliminado el toast duplicado de guardado** — Se ha corregido el comportamiento al editar un cliente: renombrar un cliente (cambiar su email) ya no provoca un error al guardar los vínculos de inbounds y los enlaces externos, pues estas operaciones ahora utilizan el nuevo email. Además, al guardar un cliente la notificación de actualización exitosa ya no aparece varias veces.

### Cambios en la sección 10 — Suscripciones (Subscription)
- **Nuevo grupo de variables de Remark Template «Connection»: {{PROTOCOL}}, {{TRANSPORT}}, {{SECURITY}}** — Al conjunto de variables de la plantilla de remark (Remark Template) se ha añadido el grupo «Conexión» (Connection) con tres variables que describen la configuración del inbound: {{PROTOCOL}} — protocolo (VLESS, VMess, Trojan, etc.), {{TRANSPORT}} — red de transporte (tcp, ws, grpc, etc.) y {{SECURITY}} — seguridad del transporte (TLS, REALITY, NONE; se muestra en mayúsculas). Al igual que las variables de consumo y plazo, estas tres variables solo actúan en el cuerpo de la suscripción y se eliminan automáticamente del remark en los enlaces mostrados en el panel y en la página de información de suscripción.
- **La plantilla de remark por defecto ahora incluye {{EMAIL}}; el email del cliente ha vuelto al remark de los enlaces del panel** — La plantilla de remark por defecto ha cambiado: ahora incluye el email del cliente — {{INBOUND}}-{{EMAIL}}|📊{{TRAFFIC_LEFT}}|⏳{{DAYS_LEFT}}D (antes el email no estaba presente). Además, se ha corregido el comportamiento de la versión 3.4.0: en los enlaces mostrados en el panel (código QR y ventanas «Información» en la página «Clientes») y en la página de información de suscripción, el email del cliente vuelve a estar presente en el nombre del perfil — «inbound-host-email» si el host está definido o «inbound-email» sin host. La información de tráfico y plazo no se incluye en estos nombres mostrados.
- **Integración del cliente Incy: botón de importación rápida y pestaña Incy con enrutamiento** — En la página de información de suscripción, en el menú de aplicaciones (Android e iOS), ha aparecido el elemento «Incy» que abre el deep-link incy://add/<enlace-de-suscripción> para la importación rápida de la suscripción en el cliente. En la configuración de suscripción se ha añadido la pestaña «Incy» con el interruptor «Activar enrutamiento» (Enable routing) y el campo «Reglas de enrutamiento» (Routing rules) en formato incy://routing/onadd/<base64>. Cuando el enrutamiento está activado y el campo está relleno, esta cadena se añade como línea independiente al cuerpo de la suscripción (formato raw), entregando el perfil de enrutamiento al cliente Incy. La configuración solo afecta al cliente Incy.
- **Restauración de {{TRAFFIC_USED}} para clientes con registro de tráfico huérfano** — Se ha corregido el cálculo de la variable {{TRAFFIC_USED}} (y otros indicadores de consumo) en el remark para los clientes cuyo registro de estadísticas de tráfico quedó «huérfano» tras eliminar y volver a crear el inbound. Anteriormente, en estos clientes {{TRAFFIC_USED}} mostraba 0.00B, aunque el consumo se mostraba correctamente en el encabezado de la página de información de suscripción. Ahora el panel busca también las estadísticas por email del cliente y la variable vuelve a mostrar el tráfico utilizado correcto.
- **Título correcto de la pestaña en la página Hosts** — En la página Hosts ahora se muestra correctamente el título de la pestaña del navegador, en lugar del genérico '3X-UI'. El cambio es puramente estético y solo afecta a la etiqueta de la pestaña.

### Cambios en la sección 11 — Xray: enrutamiento, outbounds, DNS y extensiones
- **El desplegable Dialer Proxy ahora lista los outbounds de suscripciones** — En la sección Sockopt del formulario de outbound, la lista desplegable «Dialer Proxy» (cadena de proxy: enrutar este outbound a través de otro por etiqueta) ahora muestra no solo los outbounds locales, sino también las etiquetas de outbounds de suscripciones. De la lista siguen excluidos el blackhole-outbound y el propio outbound en edición. Deje el campo vacío para una conexión directa.
- **HTTP outbound: se conservan (y editan) las cabeceras de solicitud personalizadas** — En el formulario de outbound con protocolo HTTP se ha añadido el campo «Headers» (Cabeceras) — un editor de pares clave/valor para las cabeceras CONNECT enviadas al proxy HTTP superior. Anteriormente estas cabeceras se perdían al volver a guardar el outbound; ahora se conservan. Tenga en cuenta: solo se aplican las cabeceras a nivel de configuración; las cabeceras a nivel de servidor individual las ignora xray-core.

### Cambios en la sección 12 — Nodos (multipanel, master/slave)
- **Canal Dev al actualizar nodos** — En el diálogo de confirmación de actualización de nodos ha aparecido la casilla 'Actualizar al canal de desarrollo (último commit)'. Si se marca, los nodos seleccionados instalarán la compilación rolling dev-latest en lugar del lanzamiento estable; si se desmarca, el nodo se actualiza por su canal habitual. Bajo la casilla se muestra una advertencia sobre la inestabilidad de las compilaciones dev.
- **Importación del historial de tráfico de clientes en la primera sincronización del inbound desde un nodo** — Se ha corregido el cálculo del tráfico al añadir un nodo en el que ya se había acumulado tráfico. Anteriormente, en la primera sincronización del inbound desde el nodo, el contador general del inbound se transfería correctamente, pero los contadores individuales de los clientes se ponían a cero, y el maestro subestimaba el consumo de los clientes en todo el historial anterior a la conexión del nodo. Ahora, al importar el inbound junto con el nodo, los contadores de los clientes heredan los valores reales del nodo.

### Cambios en la sección 14 — Bot de Telegram
- **Reinicio del bot de Telegram al guardar la configuración** — Los cambios en la configuración del bot de Telegram ahora se aplican inmediatamente al guardar, sin necesidad de reiniciar el panel. Si cambió el token, el chat ID, la dirección del servidor de API o activó/desactivó el bot, el panel reiniciará automáticamente el bot con los nuevos parámetros. La regla anterior sobre la necesidad de reiniciar el panel tras cambiar el token ya no está vigente.
- **Nombre del archivo de copia de seguridad del bot de Telegram — por webDomain/IP** — Los archivos de copia de seguridad de la base de datos que envía el bot de Telegram ahora se nombran según la dirección del servidor: por webDomain, y si no está definido, por la IP pública. Anteriormente, cuando webDomain no estaba definido, dichas copias recibían el nombre genérico x-ui, lo que dificultaba saber de qué servidor provenía el archivo.

### Cambios en la sección 16 — Operación: copias de seguridad, registros, actualización, CLI
- **Monitor de salud del túnel (reinicio automático de xray mediante variables de entorno)** — En 3.4.1 apareció un monitor de salud del túnel opcional. Si está activado, el panel verifica periódicamente la disponibilidad de una URL determinada y, tras varios fallos consecutivos, reinicia automáticamente el núcleo xray, lo que ayuda a recuperar el túnel que ha dejado de pasar tráfico. El monitor se configura solo mediante variables de entorno del servicio (no hay configuración en la interfaz web) y está desactivado por defecto. La variable clave XUI_TUNNEL_HEALTH_MONITOR=true lo activa; XUI_TUNNEL_HEALTH_PROXY debe apuntar al inbound local de xray (por ejemplo socks5://127.0.0.1:1080), de lo contrario solo se verifica la conectividad del propio servidor, no el túnel. Las demás variables establecen la URL de verificación (XUI_TUNNEL_HEALTH_URL), el intervalo (XUI_TUNNEL_HEALTH_INTERVAL, 30s), el tiempo de espera (XUI_TUNNEL_HEALTH_TIMEOUT, 10s), el número de fallos antes del reinicio (XUI_TUNNEL_HEALTH_FAILURES, 3) y la pausa mínima entre reinicios (XUI_TUNNEL_HEALTH_COOLDOWN, 5m). Tenga en cuenta: el reinicio de xray interrumpe las conexiones de todos los clientes conectados.
- **Actualización automática en los visores de registros** — En las ventanas de visualización de registros (tanto en «Registros de acceso» de Xray como en los «Registros» generales del panel) ha aparecido la casilla «Actualización automática». Si se activa, el registro se vuelve a leer automáticamente cada 5 segundos manteniendo el número de líneas, nivel y filtros seleccionados. El sondeo se detiene en cuanto se cierra la ventana o se desmarca la casilla.
- **Canal de actualización Dev para el panel (compilaciones rolling por commits)** — El interruptor se muestra en la ventana de actualización del panel solo en compilaciones dev (compilaciones de CI por commits individuales). Al activarlo, el panel se actualizará a la compilación rolling dev-latest, que sigue cada commit de la rama main y no es un lanzamiento estable; no hay reversión automática. En modo dev, la ventana muestra el commit actual y el último en lugar de los números de versión. La función solo está disponible en Linux con systemd.
- **Actualización al canal Dev en el menú x-ui y el comando x-ui update-dev** — En el menú de gestión del script x-ui se ha añadido el elemento de actualización al canal de desarrollo ('Update to Dev Channel (latest commit)'), que instala la compilación rolling dev-latest tras confirmación, así como el comando 'x-ui update-dev'. Por este motivo los elementos del menú han sido renumerados: ahora hay 28 elementos en total y el rango de entrada de selección es 0-28. Si el manual indica la numeración de los elementos del menú, debe verificarse de nuevo.
- **Eliminación de PostgreSQL al desinstalar el panel** — Al eliminar el panel, si este utilizaba PostgreSQL, el script ahora pregunta adicionalmente si se debe eliminar también el servidor PostgreSQL junto con todas sus bases de datos. La solicitud requiere confirmación explícita (por defecto — rechazar) y va acompañada de una advertencia: la eliminación afectará a TODAS las bases de datos de PostgreSQL en la máquina, incluidas las de otras aplicaciones, y es irreversible. Si se rechaza, PostgreSQL y sus datos se conservan.
- **El visor de registros de acceso de Xray ha sido renombrado a 'Registros de acceso'** — El visor de registros de acceso de Xray y el botón para abrirlo en la tarjeta de estado de Xray ahora se llaman 'Registros de acceso' (antes simplemente 'Registros'). Esto se ha hecho para no confundirlos con el visor general de registros del panel.
- **Selección del número de líneas de registro: se añade 1000, se elimina 10** — En ambas ventanas de registros la lista de selección del número de líneas ha cambiado: se ha eliminado el valor 10 y se ha añadido 1000. Ahora se puede seleccionar 20, 50, 100, 500 o 1000 líneas.
- **Identificador de compilación dev (dev+<commit>) en la interfaz, el bot y CLI** — En las compilaciones dev el panel muestra su versión como 'dev+<commit>' en lugar del número de versión estable — en el distintivo del panel lateral, en el panel de control, en la ventana de actualización, en el informe del bot de Telegram y en la salida de 'x-ui -v'. En los lanzamientos estables el formato de la versión no ha cambiado.
- **Visor de registros: los avisos simples se muestran tal cual, sin distorsión al formato de fecha** — El visor de registros del panel ahora muestra correctamente los avisos simples sin marca de tiempo ni nivel (por ejemplo, el mensaje del sistema 'Syslog is not supported') de forma íntegra, sin recortar el texto. Antes, estas líneas se analizaban erróneamente como una entrada de registro con fecha y nivel, y parte del texto se perdía.

## 1. Introducción, requisitos e instalación

### 1.1. Qué es 3X-UI

**3X-UI** es un panel de control web de código abierto para servidores [Xray-core](https://github.com/XTLS/Xray-core). El panel ofrece una interfaz web multilingüe unificada para desplegar, configurar y monitorear una amplia gama de protocolos de proxy y VPN: desde un solo VPS hasta configuraciones distribuidas con varios nodos.

3X-UI es un fork avanzado del proyecto original X-UI. En comparación con él, se han añadido soporte para más protocolos, mayor estabilidad, contabilidad de tráfico por cliente y numerosas funcionalidades convenientes.

Funcionalidades principales:

- **Inbound de distintos protocolos** — VLESS, VMess, Trojan, Shadowsocks, WireGuard, Hysteria2, HTTP, SOCKS (Mixed), Dokodemo-door / Tunnel, TUN y **MTProto** (proxy de Telegram, añadido en 3.3.0).
- **Transportes modernos y cifrado** — TCP (Raw), mKCP, WebSocket, gRPC, HTTPUpgrade y XHTTP, protegidos por TLS, XTLS y REALITY.
- **Fallback** — servicio de múltiples protocolos en un mismo puerto (por ejemplo, VLESS y Trojan en 443) mediante fallback en Xray.
- **Gestión por cliente** — cuotas de tráfico, fechas de vencimiento, límites de IP, indicador de estado «en línea», enlaces de invitación con un clic, códigos QR y suscripciones.
- **Estadísticas de tráfico** — por cada inbound, cliente y outbound, con posibilidad de reinicio.
- **Soporte de múltiples nodos** — gestión y escalado a varios servidores desde un único panel.
- **Outbound y enrutamiento** — WARP, NordVPN, reglas de enrutamiento personalizadas, balanceadores de carga, cadenas de proxies.
- **Servidor de suscripciones integrado** con varios formatos de salida.
- **Bot de Telegram** para monitoreo y gestión remotos.
- **REST API** con documentación Swagger integrada.
- **Almacenamiento flexible** — SQLite (por defecto) o PostgreSQL.
- **13 idiomas de interfaz**, temas claro y oscuro.
- **Integración con Fail2ban** para aplicar límites de IP por cliente.

> Importante: el proyecto está destinado únicamente al uso personal. No se recomienda utilizarlo con fines ilegales ni en entornos de producción.

### 1.2. Sistemas operativos y arquitecturas compatibles

#### Sistemas operativos

El script de instalación determina la distribución a partir del campo `ID` en `/etc/os-release` (o `/usr/lib/os-release`). Se admiten oficialmente:

Ubuntu, Debian, Armbian, Fedora, CentOS, RHEL, AlmaLinux, Rocky Linux, Oracle Linux, Amazon Linux, Virtuozzo, Arch, Manjaro, Parch, openSUSE (Tumbleweed / Leap), Alpine y Windows.

En sistemas de la familia Alpine se usa el servicio OpenRC (`rc-service` / `rc-update`); en los demás, systemd. Para CentOS 7 los paquetes se instalan mediante `yum`; para versiones más recientes, mediante `dnf`. Si la distribución no es reconocida, el script intenta usar el gestor de paquetes `apt-get` por defecto.

#### Arquitecturas de procesador

La arquitectura se determina a partir de la salida de `uname -m` y se normaliza a uno de los valores compatibles:

| Valor de `uname -m` | Arquitectura 3X-UI |
| --- | --- |
| `x86_64`, `x64`, `amd64` | `amd64` |
| `i*86`, `x86` | `386` |
| `armv8*`, `arm64`, `aarch64` | `arm64` |
| `armv7*`, `arm` | `armv7` |
| `armv6*` | `armv6` |
| `armv5*` | `armv5` |
| `s390x` | `s390x` |

Si la arquitectura no figura en esta lista, el script muestra el mensaje «Unsupported CPU architecture!» y detiene la instalación.

#### Dependencias básicas

Antes de instalar el panel, el script instala automáticamente un conjunto básico de paquetes (los nombres varían según la distribución): `cron`/`cronie`/`dcron`, `curl`, `tar`, `tzdata`/`timezone`, `socat`, `ca-certificates`, `openssl`.

### 1.3. Métodos de instalación

#### Método 1. Script de instalación (recomendado)

La instalación se realiza con un único comando como root:

```bash
bash <(curl -Ls https://raw.githubusercontent.com/mhsanaei/3x-ui/master/install.sh)
```

El script requiere obligatoriamente privilegios de root: si se ejecuta sin ellos, muestra «Please run this script with root privilege» y termina con error.

Lo que hace el instalador paso a paso:

1. Detecta el sistema operativo y la arquitectura.
2. Instala las dependencias básicas.
3. Descarga el archivo del release `x-ui-linux-<arch>.tar.gz` y lo descomprime en el directorio `/usr/local/x-ui`.
4. Descarga el script de gestión `x-ui.sh` y lo instala como el comando `/usr/bin/x-ui`.
5. Crea el directorio de logs `/var/log/x-ui`.
6. Ejecuta la configuración inicial: selección de base de datos, generación de credenciales, elección de puerto y configuración opcional de SSL.
7. Instala y activa el servicio de arranque automático (unidad systemd `x-ui.service` o script init OpenRC para Alpine).

**Selección de base de datos durante la instalación.** El instalador ofrece:

- `1) SQLite` (por defecto, recomendado para menos de 500 clientes) — un único archivo `/etc/x-ui/x-ui.db`, sin necesidad de configuración adicional.
- `2) PostgreSQL` (recomendado para un gran número de clientes o múltiples nodos). PostgreSQL puede instalarse localmente (se crea un usuario y una base de datos dedicados llamados `xui`) o puede indicarse el DSN de un servidor ya existente. Los parámetros de conexión se escriben en el archivo de entorno del servicio (`/etc/default/x-ui`, `/etc/conf.d/x-ui` o `/etc/sysconfig/x-ui` según la distribución) como las variables `XUI_DB_TYPE` y `XUI_DB_DSN`.

**Ejemplo: registro de parámetros PostgreSQL en el archivo de entorno del servicio.** Tras seleccionar PostgreSQL e indicar el DSN, el instalador añadirá al archivo de entorno unas líneas similares a estas:

```bash
XUI_DB_TYPE=postgres
XUI_DB_DSN=postgres://xui:S3cretPass@127.0.0.1:5432/xui?sslmode=disable
```

Aquí `xui` es el nombre del usuario y de la base de datos, `127.0.0.1:5432` es la dirección y el puerto del servidor, y `sslmode=disable` es adecuado para conexiones locales (para un servidor remoto se suele usar `require`).

**Instalación de una versión específica (antigua).** Es posible indicar explícitamente una etiqueta de versión; el instalador descargará el release correspondiente:

```bash
bash <(curl -Ls https://raw.githubusercontent.com/mhsanaei/3x-ui/v2.4.0/install.sh) v2.4.0
```

La versión mínima aceptable para este tipo de instalación es `v2.3.5`; si se indica una versión más antigua, se muestra «Please use a newer version (at least v2.3.5)».

**Instalación de la compilación dev.** Además de la etiqueta de versión, el instalador acepta el argumento `dev-latest` (alias `dev`), que instala la compilación rolling dev basada en el último commit de la rama `main`:

```bash
bash <(curl -Ls https://raw.githubusercontent.com/mhsanaei/3x-ui/master/install.sh) dev-latest
```

La compilación dev es un pre-release por commit (etiqueta `dev-latest`), no una versión estable, por lo que no se realiza la comprobación de versión mínima. Al ejecutarse muestra el aviso «Installing the rolling dev build (tag: dev-latest). This is a per-commit pre-release, not a stable version.». Sin argumento, el instalador instala el último release estable. Usar la compilación dev solo tiene sentido para probar correcciones aún no publicadas; en uso normal, instale versiones estables.

#### Método 2. Docker

Inicio con base de datos SQLite por defecto:

```bash
docker compose up -d
```

Para iniciar con el servicio PostgreSQL integrado, es necesario descomentar las líneas `XUI_DB_*` en `docker-compose.yml` e iniciar con el perfil:

```bash
docker compose --profile postgres up -d
```

La imagen incluye Fail2ban (activo por defecto) para aplicar límites de IP por cliente. Fail2ban bloquea a los infractores mediante `iptables`, lo que requiere la capacidad `NET_ADMIN`. En `docker-compose.yml` ya se proporciona mediante `cap_add`. Al iniciar manualmente con `docker run`, las capacidades deben añadirse manualmente; de lo contrario, los bloqueos solo se registrarán en el log pero no se aplicarán:

**Ejemplo: comando completo `docker run`.** Variante mínima con publicación del puerto del panel, capacidades de red y volumen persistente para la base de datos:

```bash
docker run -d \
  --name 3x-ui \
  --restart unless-stopped \
  --cap-add=NET_ADMIN --cap-add=NET_RAW \
  -v $PWD/db:/etc/x-ui \
  -v $PWD/cert:/root/cert \
  -p 2053:2053 \
  ghcr.io/mhsanaei/3x-ui:latest
```

El volumen `/etc/x-ui` conserva el archivo `x-ui.db` entre reinicios del contenedor; de lo contrario, la configuración y las cuentas se perderían.

```bash
docker run -d --cap-add=NET_ADMIN --cap-add=NET_RAW ... ghcr.io/mhsanaei/3x-ui
```

En Docker el panel es el proceso principal del contenedor: el arranque automático se gestiona mediante la política de reinicio del contenedor (por ejemplo, `restart: unless-stopped`), no mediante un servicio interno.

### 1.4. Primer inicio y credenciales por defecto

En la primera instalación (cuando aún se usan las credenciales por defecto), el instalador **genera valores aleatorios** para el nombre de usuario, la contraseña, la ruta web y el puerto:

| Parámetro | Cómo se genera durante la instalación | Nota |
| --- | --- | --- |
| Nombre de usuario (Username) | cadena aleatoria de 10 caracteres | se genera automáticamente |
| Contraseña (Password) | cadena aleatoria de 10 caracteres | se genera automáticamente |
| Ruta web del panel (WebBasePath) | cadena aleatoria de 18 caracteres | protege el panel de ser detectado por la URL raíz |
| Puerto del panel (Port) | por defecto, un puerto aleatorio en el rango 1024–62000; puede configurarse manualmente si se desea | el valor «de fábrica» de `webPort` es `2053`, pero el instalador lo sobrescribe |

Al final de la instalación, el script muestra un resumen: nombre de usuario, contraseña, puerto, ruta web, token de API y el enlace de acceso listo para usar (Access URL) con el formato:

```
https://<dominio-o-IP>:<puerto>/<ruta-web>
```

Si no se ha configurado un certificado SSL, el enlace será por `http://` y el script mostrará un aviso sobre la necesidad de configurar SSL (elemento de menú 19).

> Cambio obligatorio de credenciales. Dado que el login y la contraseña se generan aleatoriamente, conviene **guardarlos inmediatamente después de la instalación**. Pueden cambiarse en cualquier momento mediante el elemento de menú «Reset Username & Password» (ver más abajo) o desde la interfaz web en la configuración del panel. Tras el restablecimiento, el script recuerda: «Please use the new login username and password to access the X-UI panel. Also remember them!».

Tras la instalación, el comando `x-ui` se usa para abrir el menú de gestión (ver sección 1.6).

### 1.5. Ubicación de archivos

| Ruta | Uso |
| --- | --- |
| `/usr/local/x-ui/` | directorio de instalación del panel (binario `x-ui`, script `x-ui.sh`) |
| `/usr/local/x-ui/bin/xray-linux-<arch>` | binario de Xray-core (en armv5/armv6/armv7 se renombra a `xray-linux-arm`) |
| `/usr/bin/x-ui` | script de gestión (comando `x-ui`) |
| `/etc/x-ui/x-ui.db` | archivo de base de datos SQLite (por defecto) |
| `/var/log/x-ui/` | directorio de logs del panel |
| `/etc/systemd/system/x-ui.service` | unidad systemd del servicio (no para Alpine) |
| `/etc/init.d/x-ui` | script init OpenRC (solo Alpine) |
| `/etc/default/x-ui` · `/etc/conf.d/x-ui` · `/etc/sysconfig/x-ui` | archivo de variables de entorno del servicio (la ruta depende de la distribución); aquí se escriben `XUI_DB_TYPE`/`XUI_DB_DSN` |

El directorio de la base de datos puede redefinirse mediante la variable de entorno `XUI_DB_FOLDER` (por defecto `/etc/x-ui`), y el directorio de binarios de Xray mediante `XUI_BIN_FOLDER` (por defecto `bin` relativo al directorio del panel). El nombre del archivo de base de datos es `x-ui.db`.

**Ejemplo: mover la base de datos a un disco separado.** Para almacenar `x-ui.db` no en `/etc/x-ui` sino, por ejemplo, en un disco montado en `/data`, defina la variable en el archivo de entorno del servicio y reinicie el panel:

```bash
echo 'XUI_DB_FOLDER=/data/x-ui' >> /etc/default/x-ui
mkdir -p /data/x-ui
systemctl restart x-ui
```

La ruta completa a la base de datos será `/data/x-ui/x-ui.db`.

#### Variables de entorno principales

| Variable | Uso | Por defecto |
| --- | --- | --- |
| `XUI_DB_TYPE` | backend de BD: `sqlite` o `postgres` | `sqlite` |
| `XUI_DB_DSN` | cadena de conexión PostgreSQL (cuando `XUI_DB_TYPE=postgres`) | — |
| `XUI_DB_FOLDER` | directorio del archivo de BD SQLite | `/etc/x-ui` |
| `XUI_INIT_WEB_BASE_PATH` | ruta URI inicial de la interfaz web (solo en la primera inicialización) | `/` |
| `XUI_DB_MAX_OPEN_CONNS` | máximo de conexiones abiertas (pool PostgreSQL) | — |
| `XUI_DB_MAX_IDLE_CONNS` | máximo de conexiones inactivas (pool PostgreSQL) | — |
| `XUI_ENABLE_FAIL2BAN` | habilitar la aplicación de límites de IP mediante Fail2ban | `true` |
| `XUI_LOG_LEVEL` | nivel de registro (`debug`, `info`, `warning`, `error`) | `info` |
| `XUI_DEBUG` | modo de depuración | `false` |

**Ejemplo: habilitar el registro detallado temporalmente.** Para diagnosticar un problema, eleve el nivel de logs a `debug` y reinicie el servicio:

```bash
echo 'XUI_LOG_LEVEL=debug' >> /etc/default/x-ui
systemctl restart x-ui
x-ui log    # visualización del log de depuración
```

Tras el diagnóstico, restaure el valor `info` para evitar que el log crezca demasiado.

**Ruta inicial de la interfaz web mediante entorno.** La variable `XUI_INIT_WEB_BASE_PATH` define la ruta URI de la interfaz web (`webBasePath`) durante la inicialización inicial de la configuración. Resulta útil al desplegar en Docker o mediante systemd para fijar desde el principio la ruta de acceso al panel. El valor se normaliza automáticamente: las barras iniciales y finales se añaden si es necesario, y un valor vacío o compuesto solo de espacios se ignora (en ese caso se aplica la ruta por defecto `/`). La variable solo afecta a **la inicialización inicial**: si la configuración ya existe, la ruta se cambia desde la interfaz web o mediante el elemento de menú «Reset Web Base Path».

### 1.6. Comando de gestión `x-ui` (menú del script)

Tras la instalación, el comando `x-ui` (ejecutado como root) abre el menú interactivo «3X-UI Panel Management Script». El elemento se selecciona introduciendo su número (rango 0–27). Muchos elementos también están disponibles como subcomandos para scripts (ver sección 1.7).

El menú está dividido en bloques temáticos.

#### Instalación y actualización

- **1. Install** — instalación del panel (ejecuta `install.sh`). Antes de instalar se comprueba que el panel no esté ya instalado.
- **2. Update** — actualización de todos los componentes de x-ui a la última versión. Los datos no se pierden; tras la actualización, el panel se reinicia automáticamente. Requiere confirmación.
- **3. Update Menu** — actualización únicamente del script de gestión (`x-ui.sh` / comando `x-ui`) a la versión actual, sin reinstalar el panel.
- **4. Legacy Version** — instalación de una versión específica (antigua) del panel. El script solicita el número de versión (por ejemplo, `2.4.0`) y descarga el release correspondiente.
- **5. Uninstall** — desinstalación completa del panel **junto con Xray**. Se detiene y deshabilita el servicio, se eliminan los directorios `/etc/x-ui/` y `/usr/local/x-ui/`, el archivo de entorno del servicio y el propio script de gestión. Requiere confirmación (por defecto «no»).

#### Credenciales y configuración

- **6. Reset Username & Password** — restablecimiento del nombre de usuario y la contraseña del panel. Se pueden introducir valores propios o dejar en blanco para generación aleatoria (nombre aleatorio de 10 caracteres, contraseña aleatoria de 18 caracteres). Adicionalmente se ofrece deshabilitar la autenticación de dos factores (2FA) si está configurada. Tras el restablecimiento, el panel se reinicia.
- **7. Reset Web Base Path** — restablecimiento de la ruta web del panel: se genera una nueva ruta aleatoria (18 caracteres) y el panel se reinicia. Se usa si la ruta anterior fue comprometida u olvidada.
- **8. Reset Settings** — restablecimiento de todos los ajustes del panel a los valores por defecto. **Las credenciales (nombre de usuario y contraseña) y los datos de las cuentas no se pierden.** Requiere confirmación; tras el restablecimiento, el panel se reinicia.
- **9. Change Port** — cambio del puerto de la interfaz web. Se solicita el número de puerto (1–65535); tras configurarlo se requiere un reinicio para que el puerto entre en vigor.
- **10. View Current Settings** — visualización de la configuración actual (`x-ui setting -show`). Muestra, entre otros datos, el backend de BD utilizado (SQLite o PostgreSQL con la contraseña enmascarada en el DSN) y el enlace de acceso listo para usar (Access URL). Si no se ha configurado SSL, ofrece emitir un certificado Let's Encrypt para la dirección IP.

#### Gestión del servicio

- **11. Start** — inicio del servicio del panel. Si el panel ya está en ejecución, se muestra un mensaje indicando que no es necesario volver a iniciarlo.
- **12. Stop** — detención del servicio del panel.
- **13. Restart** — reinicio del servicio del panel.
- **14. Restart Xray** — reinicio únicamente del núcleo Xray-core sin reiniciar el panel (mediante `systemctl reload x-ui`; en Docker, con la señal `USR1` al proceso del panel).
- **15. Check Status** — comprobación del estado del servicio (`systemctl status x-ui` o `rc-service x-ui status`).
- **16. Logs Management** — gestión de logs: visualización del log de depuración (Debug Log, mediante `journalctl`) y, salvo en Alpine, limpieza de todos los logs (Clear All logs).

#### Arranque automático

- **17. Enable Autostart** — habilitar el inicio automático del panel al arrancar el sistema operativo (`systemctl enable x-ui` o `rc-update add`).
- **18. Disable Autostart** — deshabilitar el inicio automático al arrancar el sistema operativo.

En Docker el arranque automático se gestiona mediante la política de reinicio del contenedor, por lo que estos elementos simplemente muestran el mensaje de ayuda correspondiente.

#### Seguridad y red

- **19. SSL Certificate Management** — gestión de certificados SSL mediante acme.sh: emisión de certificados para un dominio, revocación, renovación forzada, visualización de dominios existentes, indicación de rutas al certificado para el panel, así como emisión de un certificado de corta duración (~6 días, con renovación automática) para una dirección IP.
- **20. Cloudflare SSL Certificate** — emisión de un certificado SSL mediante validación DNS de Cloudflare.
- **21. IP Limit Management** — gestión de límites de número de IP por cliente (basado en Fail2ban): visualización y eliminación de bloqueos, etc.
- **22. Firewall Management** — gestión del cortafuegos (apertura/cierre de puertos y visualización de reglas).
- **23. SSH Port Forwarding Management** — configuración del reenvío de puertos SSH para acceder al panel desde una máquina local mediante túnel SSH.

#### Rendimiento y mantenimiento

- **24. Enable BBR** — activación/desactivación del algoritmo de control de congestión TCP BBR (submenú con los elementos Enable BBR / Disable BBR).
- **25. Update Geo Files** — actualización de las bases de datos geo (archivos `.dat`) con selección de fuente: Loyalsoldier (`geoip.dat`, `geosite.dat`), chocolate4u (`geoip_IR.dat`, `geosite_IR.dat`), runetfreedom (`geoip_RU.dat`, `geosite_RU.dat`) o All (todas a la vez). Tras la actualización, el panel se reinicia.
- **26. Speedtest by Ookla** — ejecución del test de velocidad de red mediante Speedtest by Ookla.
- **27. PostgreSQL Management** — gestión de la instancia PostgreSQL integrada/vinculada (habilitación y operaciones relacionadas).
- **0. Exit Script** — salir del menú.

### 1.7. Subcomandos de `x-ui` (sin menú interactivo)

Para uso en scripts, el comando `x-ui` admite subcomandos directos (ejecutar `x-ui` sin argumentos abre el menú):

| Comando | Acción |
| --- | --- |
| `x-ui` | abrir el menú de gestión |
| `x-ui start` | iniciar el panel |
| `x-ui stop` | detener el panel |
| `x-ui restart` | reiniciar el panel |
| `x-ui restart-xray` | reiniciar Xray |
| `x-ui status` | estado actual del servicio |
| `x-ui settings` | configuración actual |
| `x-ui enable` | habilitar el inicio automático al arrancar el sistema operativo |
| `x-ui disable` | deshabilitar el inicio automático |
| `x-ui log` | visualización de logs |
| `x-ui banlog` | visualización de logs de bloqueos de Fail2ban |
| `x-ui update` | actualizar el panel |
| `x-ui update-all-geofiles` | actualizar todos los archivos geo |
| `x-ui migrateDB [file]` | conversión `.db` ↔ `.dump` (SQLite) |
| `x-ui legacy` | instalar una versión antigua |
| `x-ui install` | instalar el panel |
| `x-ui uninstall` | desinstalar el panel |

### 1.8. Migración SQLite → PostgreSQL

Una instalación existente en SQLite puede migrarse a PostgreSQL:

```bash
x-ui migrate-db --dsn "postgres://xui:password@127.0.0.1:5432/xui?sslmode=disable"
# luego definir XUI_DB_TYPE y XUI_DB_DSN en /etc/default/x-ui y reiniciar:
systemctl restart x-ui
```

El archivo SQLite de origen permanece intacto — elimínelo manualmente solo después de verificar el correcto funcionamiento del nuevo backend.

**Ejemplo: verificación del cambio a PostgreSQL.** Tras la migración, compruebe que el panel efectivamente trabaja con el nuevo backend mediante el comando de visualización de configuración — la salida debe indicar PostgreSQL (la contraseña en el DSN se enmascara):

```bash
x-ui settings | grep -i -E 'db|dsn'
```

Si el panel se abre y las cuentas están presentes, el archivo `x-ui.db` original puede eliminarse.

---

## 2. Acceso al panel y seguridad

Este apartado describe todo lo relativo a la autenticación del administrador del panel 3X-UI: el formulario de inicio de sesión, la autenticación de doble factor (TOTP), la protección contra fuerza bruta, el cambio de credenciales, la modificación de la ruta secreta y el puerto del panel, el tiempo de vida de la sesión, así como la sincronización y autenticación mediante LDAP.

### 2.1. Formulario de inicio de sesión

La página de inicio de sesión se sirve en la raíz de la ruta secreta del panel (`webBasePath`). Si el usuario ya está autenticado, es redirigido automáticamente a `…/panel/`. La página incluye un selector de tema, un selector de idioma de interfaz y el propio formulario.

Campos del formulario:

| Campo | Etiqueta/encabezado | Obligatorio | Descripción |
|-------|---------------------|-------------|-------------|
| Nombre de usuario | «Nombre de usuario» | Sí | Nombre de usuario del administrador. Un valor vacío se rechaza en el cliente y, en el servidor, con el mensaje «Introduzca el nombre de usuario». |
| Contraseña | «Contraseña» | Sí | Contraseña del administrador. Un valor vacío se rechaza con el mensaje «Introduzca la contraseña». |
| Código 2FA | «Código 2FA» | Solo si 2FA está activado | El campo aparece **únicamente** si la autenticación de doble factor está habilitada en el panel. Código de 6 dígitos generado por la aplicación autenticadora. |

El botón **«Iniciar sesión»** envía el formulario a `POST /login`.

Comportamiento y mensajes:

- Tras un inicio de sesión exitoso se muestra «Sesión iniciada correctamente» y se produce la redirección a `…/panel/`.
- Ante cualquier error de credenciales o código 2FA incorrecto, el servidor devuelve un mensaje **unificado**: «Datos de cuenta incorrectos.» (en inglés: *Invalid username or password or two-factor code.*). Esto es intencional: el panel no indica qué dato es incorrecto (usuario, contraseña o código) para no facilitar los ataques de fuerza bruta.
- El campo «Código 2FA» se muestra u oculta en función de la respuesta a `POST /getTwoFactorEnable`, que devuelve el estado actual de 2FA incluso antes de que el usuario se autentique.
- Si la sesión del servidor ha expirado, el siguiente request muestra «La sesión ha expirado. Vuelva a iniciar sesión» y el usuario es redirigido a la página de inicio de sesión.

> Nota sobre CSRF: antes de enviar el formulario, el cliente obtiene un token CSRF (`GET /csrf-token`); las rutas `/login` y `/logout` están protegidas con verificación CSRF.

**Ejemplo: inicio de sesión a través de la API.** Cuando 2FA está desactivado basta con el nombre de usuario y la contraseña; si 2FA está activado se añade el campo `twoFactorCode`:

```bash
# Sin 2FA
curl -i -X POST https://panel.example.com:2053/мой-секрет/login \
  -H 'Content-Type: application/x-www-form-urlencoded' \
  --data 'username=admin&password=ВашПароль'

# Con 2FA activado — se añade el código de 6 dígitos
curl -i -X POST https://panel.example.com:2053/мой-секрет/login \
  -H 'Content-Type: application/x-www-form-urlencoded' \
  --data 'username=admin&password=ВашПароль&twoFactorCode=123456'
```

Si la operación tiene éxito, el servidor devolverá `Set-Cookie` con la cookie de sesión, que deberá enviarse en las peticiones posteriores a `/panel/api/…`.

### 2.2. Autenticación de doble factor (2FA / TOTP)

La 2FA en 3X-UI está implementada según el estándar **TOTP** y es compatible con cualquier aplicación autenticadora (Google Authenticator, Aegis, FreeOTP, etc.). Los parámetros están fijados en el código: algoritmo **SHA1**, **6** dígitos, período de **30** segundos, emisor (issuer) `3x-ui`, etiqueta `Administrator`.

**Ejemplo: URI otpauth codificada en el código QR.** Si la aplicación autenticadora no puede leer la cámara, el token puede añadirse manualmente con el siguiente enlace (sustituya su secreto en Base32 por `JBSWY3DPEHPK3PXP`):

```
otpauth://totp/3x-ui:Administrator?secret=JBSWY3DPEHPK3PXP&issuer=3x-ui&algorithm=SHA1&digits=6&period=30
```

Los parámetros `algorithm=SHA1`, `digits=6`, `period=30` corresponden a los valores fijos del panel y no deben modificarse.

La configuración se encuentra en **Configuración → Cuenta de usuario**, pestaña **«Autenticación de doble factor»**.

| Elemento | Texto | Descripción |
|----------|-------|-------------|
| Interruptor | «Activar 2FA» | Activa o desactiva la autenticación de doble factor. |
| Descripción | «Añade un nivel adicional de autenticación para mayor seguridad.» | Texto de ayuda bajo el interruptor. |

#### Cómo activar 2FA

Al activar el interruptor, el panel **genera localmente un nuevo secreto**: una cadena aleatoria en codificación Base32 (alfabeto `A–Z` y `2–7`). Se abre la ventana «Activar autenticación de doble factor» con instrucciones paso a paso:

1. **«Escanee este código QR en la aplicación autenticadora o copie el token que aparece junto al código QR e introdúzcalo en la aplicación»**. Bajo el código QR se muestra el secreto en texto plano; al hacer clic en el QR se copia al portapapeles (aparece «Copiado»).
2. **«Introduzca el código de la aplicación»**: hay que introducir el código de 6 dígitos generado por la aplicación. El código se verifica **en el navegador**: el panel calcula el TOTP actual con el secreto recién generado y lo compara con el introducido. Si es incorrecto, muestra «Código incorrecto»; el campo solo acepta exactamente 6 dígitos.

Solo tras la confirmación exitosa se guardan el secreto y el indicador de activación. Al guardar se muestra «La autenticación de doble factor se ha configurado correctamente».

Importante: los cambios en la sección de configuración se aplican con el botón general **«Guardar»**, tras lo cual normalmente es necesario reiniciar el panel («Guarde los cambios y reinicie el panel para que surtan efecto»). Al activar 2FA por primera vez, el servidor adicionalmente **invalida todas las sesiones activas** (incrementa el «login epoch»), por lo que después de aplicar la configuración será necesario volver a iniciar sesión, esta vez con el código 2FA.

#### Cómo desactivar 2FA

Al volver a pulsar el interruptor se abre la ventana «Desactivar autenticación de doble factor» con el texto «Introduzca el código de la aplicación para desactivar la autenticación de doble factor.». Tras introducir el código correcto, el indicador y el secreto se borran, y se muestra «La autenticación de doble factor se ha eliminado correctamente».

#### Verificación del código al iniciar sesión

Al iniciar sesión, el servidor toma el secreto almacenado y compara el TOTP actual con el código 2FA enviado. Si no coinciden, el intento se considera fallido, pero al usuario se le muestra el mismo mensaje unificado «Datos de cuenta incorrectos.».

#### Recuperación de acceso (recovery)

3X-UI **no** dispone de un mecanismo de «códigos de recuperación». Si se pierde el acceso a la aplicación autenticadora, no es posible recuperar el inicio de sesión desde la interfaz del panel. La única vía es desactivar 2FA directamente en la base de datos del servidor: restablecer la clave `twoFactorEnable` a `false` (y si es necesario borrar `twoFactorToken`) en la tabla de configuración y reiniciar el panel. Por eso se recomienda guardar el secreto (token Base32) en un lugar seguro al activar 2FA.

**Ejemplo: desactivación de emergencia de 2FA en el servidor.** Con acceso SSH al servidor, detenga el panel, restablezca las claves en la tabla de configuración e inícielo de nuevo:

```bash
x-ui stop
sqlite3 /etc/x-ui/x-ui.db "UPDATE settings SET value='false' WHERE key='twoFactorEnable';"
sqlite3 /etc/x-ui/x-ui.db "UPDATE settings SET value='' WHERE key='twoFactorToken';"
x-ui start
```

Después de esto, el acceso se realiza solo con nombre de usuario y contraseña, y si se desea puede configurarse 2FA de nuevo.

> Relación con el cambio de credenciales: al cambiar el nombre de usuario o la contraseña (véase 2.4), 2FA **se desactiva automáticamente** en el servidor para que el antiguo secreto no bloquee el acceso con la nueva cuenta.

### 2.3. Limitación de intentos de inicio de sesión (login limiter / protección contra fuerza bruta)

El panel incluye un limitador de intentos de inicio de sesión fallidos integrado (equivalente a fail2ban a nivel de aplicación). Los parámetros están fijados en el código y **no son configurables** desde la interfaz:

| Parámetro | Valor | Función |
|-----------|-------|---------|
| Máximo de fallos | **5** | Número de intentos fallidos permitidos dentro de la ventana. |
| Ventana de conteo | **5 minutos** | Ventana deslizante en la que se acumulan los fallos (los más antiguos se descartan). |
| Bloqueo (cooldown) | **15 minutos** | Tiempo durante el que la clave queda bloqueada tras superar el umbral. |

Funcionamiento:

- La clave de bloqueo se construye a partir de la **combinación «IP + nombre de usuario»** (el nombre de usuario se convierte a minúsculas y se eliminan los espacios). Es decir, el bloqueo se aplica al par concreto «dirección + nombre de usuario», no al panel en su conjunto.
- Con cada intento fallido (usuario/contraseña incorrectos o código 2FA incorrecto) el contador aumenta. Al alcanzar **5** fallos en **5 minutos**, la clave queda bloqueada durante **15 minutos**. Durante el bloqueo, cualquier intento de ese par se rechaza inmediatamente con el mismo mensaje «Datos de cuenta incorrectos.», aunque las credenciales sean correctas.
- **Un inicio de sesión exitoso restablece de inmediato** el contador y elimina el bloqueo para ese par.
- La dirección IP del cliente se determina teniendo en cuenta los proxies de confianza (véase `trustedProxyCIDRs`): las cabeceras `X-Real-IP` y `X-Forwarded-For` solo se aceptan si la petición proviene de una dirección de confianza. De lo contrario se usa la dirección real de la conexión y, si no puede obtenerse, la cadena `unknown`.

Todos los intentos se registran en el log. Los intentos fallidos generan una advertencia en el log del servidor con el nombre de usuario, la IP, el motivo y, en caso de bloqueo, el tiempo `blocked_until`. Si las notificaciones de inicio de sesión están habilitadas a través del bot de Telegram (`tgNotifyLogin` — «Notificación de inicio de sesión»), el administrador recibe adicionalmente el nombre de usuario, la IP y la hora de cada intento: exitoso, fallido o bloqueado.

**Ejemplo: notificación de inicio de sesión en Telegram.** Con `tgNotifyLogin` activado, tras cada intento el administrador recibe un mensaje similar al siguiente:

```
Уведомление о входе
Пользователь: admin
IP: 203.0.113.45
Время: 2026-06-10 14:32:07
Статус: успешно
```

Para el par «IP + nombre de usuario» bloqueado, el estado indicará que el intento fue rechazado por el limitador.

### 2.4. Cambio de nombre de usuario y contraseña del administrador

Sección **Configuración → Cuenta de usuario**, pestaña **«Credenciales de administrador»**. Campos:

| Campo | Texto | Descripción |
|-------|-------|-------------|
| Usuario actual | «Usuario actual» | Nombre de usuario en uso. Debe coincidir con el nombre de usuario actual; de lo contrario, el cambio se rechaza. |
| Contraseña actual | «Contraseña actual» | Contraseña en uso para verificar la identidad. |
| Nuevo usuario | «Nuevo usuario» | Nuevo nombre de usuario. No puede estar vacío. |
| Nueva contraseña | «Nueva contraseña» | Nueva contraseña. No puede estar vacía. |

El cambio se aplica con el botón **«Confirmar»** y se envía a `POST /panel/setting/updateUser`.

Lógica y mensajes del servidor:

- Si «Usuario actual» no coincide con el real o «Contraseña actual» es incorrecta: «Se produjo un error al cambiar las credenciales del administrador.» con la aclaración «Nombre de usuario o contraseña incorrectos».
- Si el nuevo nombre de usuario o la nueva contraseña están vacíos: «El nuevo nombre de usuario y la nueva contraseña deben estar rellenos».
- Si la operación es exitosa: «Ha cambiado correctamente las credenciales del administrador.». La contraseña se almacena como hash bcrypt.

**Ejemplo: cambio de credenciales a través de la API.** La petición requiere una cookie de sesión válida (obtenida al iniciar sesión) y la confirmación del nombre de usuario y contraseña actuales:

```bash
curl -X POST https://panel.example.com:2053/мой-секрет/panel/setting/updateUser \
  -b 'session=ВАША_СЕССИОННАЯ_COOKIE' \
  -H 'Content-Type: application/x-www-form-urlencoded' \
  --data 'oldUsername=admin&oldPassword=СтарыйПароль&newUsername=root&newPassword=НовыйСложныйПароль'
```

Tras el éxito, la sesión actual se invalida y será necesario volver a iniciar sesión con las nuevas credenciales.

Efectos importantes del cambio de credenciales:

- **Todas las sesiones existentes se invalidan** (se incrementa el contador `login_epoch` del usuario), por lo que tras el cambio el panel cierra la sesión automáticamente y redirige a la página de inicio de sesión: habrá que volver a autenticarse.
- Si en el momento del cambio **2FA estaba activado, se desactiva automáticamente** (el indicador y el secreto se borran). Será necesario configurar la autenticación de doble factor de nuevo tras el cambio de nombre de usuario y contraseña.

Si 2FA está activado, antes de enviar el formulario se abre la ventana «Cambiar credenciales» con el texto «Introduzca el código de la aplicación para cambiar las credenciales del administrador.»: solo es posible cambiar las credenciales tras confirmar el código 2FA actual.

### 2.5. Ruta secreta (ruta URI / webBasePath) y puerto del panel

Estos parámetros se encuentran en **Configuración → Panel** y afectan directamente a la «visibilidad» y accesibilidad del panel. Se aplican tras guardar y **reiniciar el panel**.

| Campo | Texto | Valor por defecto | Descripción |
|-------|-------|-------------------|-------------|
| Puerto del panel | «Puerto del panel» (`panelPort`), ayuda «Puerto en el que opera el panel» | **2053** | Puerto TCP de la interfaz web. |
| URI-путь | «URI-путь» (`panelUrlPath`), ayuda «Debe comenzar con '/' y terminar con '/'» | **/** | Ruta base secreta (`webBasePath`). El panel solo es accesible a través de ella (por ejemplo, `/mi-secreto/`). |
| Dirección IP para administrar el panel | «Dirección IP para administrar el panel» (`panelListeningIP`), ayuda «Déjelo vacío para permitir conexiones desde cualquier IP» | vacío | Dirección en la que escucha el panel. Vacío = todas las interfaces. |
| Dominio del panel | «Dominio del panel» (`panelListeningDomain`), ayuda «Déjelo vacío para permitir conexiones desde cualquier dominio e IP.» | vacío | Restricción de acceso por dominio (Host). |
| Ruta al certificado público del panel | `publicKeyPath`, ayuda «Introduzca la ruta completa comenzando con '/'» | vacío | Certificado TLS para el acceso HTTPS al panel. |
| Ruta a la clave privada del certificado del panel | `privateKeyPath`, misma ayuda | vacío | Clave privada TLS. |

Comportamiento de la ruta base (`webBasePath`):

- El valor se normaliza automáticamente: si no comienza con `/`, el carácter se añade al inicio; si no termina con `/`, se añade al final. Es decir, la ruta siempre tiene la forma `/…/`.
- La ruta base se aplica al propio panel, a los assets y a la cookie de sesión (la cookie solo se emite para esta ruta).

> Recomendaciones de seguridad (sección «Avisos de seguridad»): el panel muestra avisos si la configuración es «demasiado pública»:
> - «El panel opera sobre HTTP sin cifrar — configure TLS para producción.»
> - «El puerto estándar 2053 es ampliamente conocido — cámbielo por uno aleatorio.»
> - «La ruta base por defecto "/" es ampliamente conocida — cámbiela por una aleatoria.»
>
> En otras palabras, para un servidor en producción conviene definir un **puerto no estándar**, una **ruta URI no trivial** y un **certificado TLS**.

**Ejemplo: configuración «discreta» del panel para producción.** En **Configuración → Panel** introduzca valores similares a los siguientes:

| Campo | Valor |
|-------|-------|
| Puerto del panel | `34571` (aleatorio, en lugar de 2053) |
| URI-путь | `/aXf9Qm2/` (no trivial, comienza y termina con `/`) |
| Ruta al certificado público del panel | `/etc/letsencrypt/live/panel.example.com/fullchain.pem` |
| Ruta a la clave privada del certificado del panel | `/etc/letsencrypt/live/panel.example.com/privkey.pem` |

Tras guardar y reiniciar, el panel solo será accesible en `https://panel.example.com:34571/aXf9Qm2/` y los avisos de seguridad desaparecerán.

### 2.6. Tiempo de vida de la sesión (timeout)

El campo **«Duración de la sesión»** (`sessionMaxAge`) se encuentra entre los ajustes del panel e intervalos.

| Campo | Texto | Valor por defecto | Unidad | Descripción |
|-------|-------|-------------------|--------|-------------|
| Duración de la sesión | «Duración de la sesión», ayuda «Duración de la sesión en el sistema (valor: minuto)» | **360** | minutos | Tiempo de vida de la cookie de sesión del administrador. |

Comportamiento:

- El valor se especifica en **minutos** (por defecto 360 minutos = 6 horas) y se convierte a segundos al configurar la cookie.
- Si el valor es **mayor que 0**, la cookie de sesión recibe el `MaxAge` correspondiente. Una vez transcurrido ese tiempo, la cookie deja de ser válida y en la siguiente petición el usuario recibe «La sesión ha expirado. Vuelva a iniciar sesión».
- La sesión también se invalida antes de tiempo al cambiar las credenciales o al activar 2FA por primera vez (mediante el mecanismo `login_epoch`, véase 2.4 y 2.2) y al cerrar sesión explícitamente (`POST /logout`).
- La cookie de sesión se marca como `HttpOnly`, con política `SameSite=Lax`; el indicador `Secure` se activa cuando el acceso al panel es directamente por HTTPS.

Además del propio timeout existe una notificación relacionada: **«Retraso de notificación de expiración de sesión»** (`expireTimeDiff`, ayuda «Recibir una notificación sobre la expiración de la sesión antes de alcanzar el valor umbral (valor: día)», por defecto `0`) — permite recibir un aviso con antelación.

### 2.7. LDAP (sincronización y autenticación)

La sección LDAP ofrece dos posibilidades: (1) autenticar el inicio de sesión del administrador mediante LDAP si la contraseña local no coincide, y (2) sincronizar periódicamente el estado de los clientes (indicador VLESS activado/desactivado) desde el directorio.

Uso en el inicio de sesión: el servidor comprueba primero el hash bcrypt local de la contraseña. Si **no coincide** y LDAP está activado, el panel intenta autenticar al usuario en el directorio: si se ha definido un `Bind DN`, se realiza un bind de servicio y se busca la entrada del usuario con el filtro y el atributo indicados; a continuación se intenta un bind con el DN encontrado y la contraseña introducida. Si tiene éxito, se concede el acceso. (Tras una autenticación LDAP exitosa, si 2FA está activado se comprueba igualmente el código TOTP.)

Campos de la sección:

| Campo | Texto | Valor por defecto | Descripción |
|-------|-------|-------------------|-------------|
| Activar sincronización LDAP | «Activar sincronización LDAP» (`enable`) | **false** | Interruptor principal de la integración LDAP. |
| Host LDAP | «Host LDAP» (`host`) | vacío | Dirección del servidor LDAP. |
| Puerto LDAP | «Puerto LDAP» (`port`) | **389** | Puerto. Para LDAPS normalmente 636. |
| Usar TLS (LDAPS) | «Usar TLS (LDAPS)» (`useTls`) | **false** | Al activarlo se usa el esquema `ldaps://` con verificación del certificado del servidor (sin omitir la verificación). |
| Bind DN | «Bind DN» (`bindDn`) | vacío | DN de la cuenta de servicio para el bind/búsqueda inicial. Si está vacío, no se realiza bind (búsqueda anónima). |
| Contraseña de bind | ayudas: «Configurado; déjelo vacío para conservar la contraseña actual.» / «No configurado.» / «Configurado — introduzca un nuevo valor para sustituirlo» | vacío | Contraseña para `Bind DN`. Se almacena por separado; para conservar la anterior, se deja el campo vacío. |
| Base DN | «Base DN» (`baseDn`) | vacío | Raíz del subárbol en el que se realiza la búsqueda (búsqueda recursiva en todo el subárbol). |
| Filtro de usuario | «Filtro de usuario» (`userFilter`) | `(objectClass=person)` | Filtro LDAP para seleccionar cuentas. Durante la autenticación, el nombre de usuario se sustituye en el filtro con escape. |
| Atributo de usuario (username/email) | «Atributo de usuario (username/email)» (`userAttr`) | `mail` | Atributo que se compara con el nombre de usuario/identificador del cliente (por ejemplo, `mail` o `uid`). |
| Atributo del indicador VLESS | «Atributo del indicador VLESS» (`vlessField`) | `vless_enabled` | Atributo que determina si el acceso VLESS del cliente debe estar activado. |
| Atributo de indicador general (opc.) | «Atributo de indicador general (opc.)» (`flagField`), ayuda «Si se define, reemplaza al indicador VLESS — p. ej. shadowInactive.» | vacío | Si se define, se usa en lugar de `vless_enabled`. |
| Valores verdaderos | «Valores verdaderos» (`truthyValues`), ayuda «Separados por coma; por defecto: true,1,yes,on» | `true,1,yes,on` | Lista de valores del atributo de indicador que se interpretan como «activado». |
| Invertir indicador | «Invertir indicador» (`invertFlag`), ayuda «Actívelo cuando el atributo signifique «desactivado» (p. ej. shadowInactive).» | **false** | Invierte el significado del indicador. |
| Programación de sincronización | «Programación de sincronización» (`syncSchedule`), ayuda «Cadena tipo cron, p. ej. @every 1m» | `@every 1m` | Frecuencia de sincronización en formato similar a cron. |
| Etiquetas de inbounds | «Etiquetas de inbounds» (`inboundTags`), ayuda «Inbounds en los que la sincronización LDAP puede crear o eliminar clientes automáticamente.» | vacío | Limita en qué inbounds están permitidas las operaciones automáticas. Si no hay inbounds: «No se encontraron inbounds. Cree primero un inbound.» |
| Creación automática de clientes | «Creación automática de clientes» (`autoCreate`) | **false** | Crear un cliente en los inbounds indicados si aparece en el directorio. |
| Eliminación automática de clientes | «Eliminación automática de clientes» (`autoDelete`) | **false** | Eliminar un cliente si desaparece del directorio. |
| Volumen por defecto (GB) | «Volumen por defecto (GB)» (`defaultTotalGb`) | **0** | Límite de tráfico para los clientes creados automáticamente (0 = sin límite). |
| Plazo por defecto (días) | «Plazo por defecto (días)» (`defaultExpiryDays`) | **0** | Período de validez para los clientes creados automáticamente (0 = sin expiración). |
| Límite de IP por defecto | «Límite de IP por defecto» (`defaultIpLimit`) | **0** | Límite de IPs simultáneas (0 = sin límite). |

Detalles de la lógica del indicador de sincronización: al leer el atributo de indicador (`flagField`, por defecto `vless_enabled`), el valor se considera «activado» si pertenece a la lista de valores verdaderos; si la inversión está habilitada, el resultado se invierte. El atributo de usuario (`userAttr`) se usa como clave de correspondencia (email/nombre); las entradas sin valor en ese atributo se omiten.

> Seguridad: se recomienda activar **TLS (LDAPS)** para que las contraseñas de bind y las contraseñas verificadas no se transmitan en texto plano, y usar para `Bind DN` una cuenta con los permisos mínimos necesarios de lectura.

**Ejemplo: configuración típica de sincronización LDAP (Active Directory).** Valores de los campos para un directorio donde el estado de acceso se almacena en un atributo similar a `userAccountControl` y la correspondencia se realiza por correo electrónico:

| Campo | Valor |
|-------|-------|
| Host LDAP | `ldap.example.com` |
| Puerto LDAP | `636` |
| Usar TLS (LDAPS) | activado |
| Bind DN | `CN=svc-3xui,OU=Service,DC=example,DC=com` |
| Base DN | `OU=Users,DC=example,DC=com` |
| Filtro de usuario | `(objectClass=person)` |
| Atributo de usuario (username/email) | `mail` |
| Atributo del indicador VLESS | `vless_enabled` |
| Valores verdaderos | `true,1,yes,on` |
| Programación de sincronización | `@every 5m` |

Con esta configuración, cada 5 minutos el panel recorrerá el subárbol `OU=Users`, emparejará los clientes por `mail` y activará o desactivará el acceso VLESS según el valor de `vless_enabled`.

---

## 3. Resumen / Dashboard

El Dashboard (*Overview* en la interfaz en inglés) es la página de inicio del panel. Muestra en tiempo real el estado del servidor y del proceso Xray. Todos los indicadores provienen del lado del servidor. El planificador en segundo plano reconstruye el snapshot **cada 2 segundos** y lo distribuye a todas las pestañas abiertas mediante WebSocket; una vez por minuto, las filas de métricas acumuladas se vuelcan al disco. El endpoint HTTP `GET /status` devuelve el último snapshot cacheado.

A continuación se describe cada indicador y cada elemento de control de la página.

### 3.1. Principios generales de recolección de datos

- El snapshot se recopila mediante la biblioteca `gopsutil`. Si una medición concreta falla, el campo queda en cero y se escribe una advertencia en el log (`get cpu percent failed`, `get uptime failed`, etc.) — esto no tumba el dashboard completo, simplemente el bloque correspondiente mostrará 0/N-A.
- Las velocidades «instantáneas» (CPU %, red, I/O de disco) se calculan como la diferencia entre el snapshot actual y el anterior, dividida entre el intervalo en segundos. Por eso, al cargar la página por primera vez, los valores de velocidad pueden ser cero hasta que se acumule una segunda medición.
- El historial puede consultarse en la sección «Historia del sistema» (*System History*) — los gráficos se construyen con las mismas filas de datos descritas a continuación (véase punto 3.12).

### 3.2. CPU

El bloque «CPU» (*CPU*) muestra la carga actual del procesador en porcentaje, así como los parámetros del propio procesador.

| Indicador | Descripción |
|---|---|
| Carga de CPU, % | Fracción del tiempo de procesador utilizado durante el último intervalo. Se suaviza mediante una media exponencial (EMA, coeficiente `alpha = 0.3`) para evitar que los picos sacudan el indicador. El valor siempre está limitado al rango 0–100 %. En la primera medición se devuelve 0 (inicialización del punto base). |
| Procesadores lógicos | Número de núcleos lógicos, es decir, contando Hyper-Threading. |
| Núcleos físicos | Número de núcleos físicos. |
| Frecuencia | Frecuencia base del procesador en MHz. Se solicita de forma diferida y se cachea: la primera medición exitosa se guarda, el reintento no se realiza más de una vez cada 5 minutos, y la propia solicitud tiene un timeout de 1,5 s (en algunos sistemas la consulta de frecuencia responde lentamente). |

El cálculo algorítmico de la carga de CPU funciona así: si existe una implementación nativa para la plataforma, se usa; de lo contrario, se calcula mediante deltas de los contadores de tiempo de procesador (busy / total). El tiempo Guest y GuestNice se excluye para no contabilizarlo dos veces.

### 3.3. Memoria (RAM)

El bloque «Memoria» (*RAM*) muestra la memoria usada y el total. Se presenta como «usado / total» y/o como porcentaje de ocupación. En el historial se registra el porcentaje.

### 3.4. Swap

El bloque «Swap» (*Swap*) muestra la memoria de intercambio usada y el total. Si no hay archivo/partición de swap configurado (total = 0), el indicador es cero; en la fila histórica se escribe 0 cuando no hay swap.

### 3.5. Disco (Storage)

El bloque «Disco» (*Storage*) muestra el espacio usado y el total, teniendo en cuenta **únicamente la partición raíz `/`**. En el historial «Uso de disco» (*Disk Usage*) se registra el porcentaje de ocupación. Adicionalmente se recopila el I/O de disco (lectura / escritura, bytes/s) como delta de los contadores por intervalo — se muestra en la pestaña «Disco I/O» del historial.

### 3.6. Tiempo de actividad del sistema (Uptime)

El indicador «Tiempo de actividad del sistema» (*Uptime*) es el tiempo transcurrido desde el arranque **del servidor completo** (en segundos), no el tiempo de ejecución del panel o de Xray. Se almacena por separado el uptime del proceso Xray (véase punto 3.9), así como el número de hilos del panel (en la interfaz, «Hilos» / *Threads*).

#### Memoria consumida por el panel

Junto a los indicadores del proceso del panel se muestra la cantidad de memoria RAM que ocupa el propio proceso 3X-UI. Este valor se obtiene del RSS real del proceso (tal como lo ve el sistema operativo) y coincide con lo que muestran las utilidades del sistema. El número disminuye a medida que se libera memoria. Anteriormente el panel mostraba un contador interno de Go que sobreestimaba el consumo de memoria (por ejemplo, ~300 MB en un servidor inactivo con un solo cliente) y nunca disminuía — ese artefacto ya no existe. Adicionalmente, un proceso periódico en segundo plano devuelve la memoria no utilizada al sistema operativo para que el indicador refleje el consumo real.

### 3.7. Carga del sistema (Load average)

El bloque «Carga del sistema» (*System Load*) es un array de tres números `[Load1, Load5, Load15]`. Descripción: «Carga media del sistema durante los últimos 1, 5 y 15 minutos» (*System load average for the past 1, 5, and 15 minutes*). El gráfico de historial se llama «Carga media del sistema (1 / 5 / 15 min)». Los valores se escriben en filas históricas por separado: `load1`, `load5`, `load15`.

Este es el indicador Unix estándar: el número medio de procesos en cola de ejecución. La referencia es compararlo con el número de núcleos: una carga que supera de forma sostenida la cantidad de núcleos físicos indica sobrecarga.

### 3.8. Red: velocidad y volumen total de tráfico

Se tienen en cuenta **únicamente las interfaces físicas**. Las interfaces virtuales y de túnel se excluyen: `lo`/`lo0`, y todo lo que comienza por `loopback`, `docker`, `br-`, `veth`, `virbr`, `tun`, `tap`, `wg`, `tailscale`, `zt`. Los valores se suman para todas las interfaces restantes.

**Velocidad general** (*Overall Speed*) — velocidad instantánea, delta de contadores por intervalo:

| Indicador | Descripción |
|---|---|
| Subida / envío (etiqueta «Upload») | Velocidad de salida, bytes/s. |
| Bajada / recepción (etiqueta «Download») | Velocidad de entrada, bytes/s. |

**Volumen total de tráfico** (*Total Data*) — contadores acumulados desde el inicio del sistema:

| Indicador | Descripción |
|---|---|
| Enviado (etiqueta «Sent») | Total de bytes enviados. |
| Recibido (etiqueta «Received») | Total de bytes recibidos. |

Adicionalmente se recopilan las velocidades en paquetes (paquetes/s) y los contadores totales de paquetes — se muestran en la pestaña «Paquetes de red» (*Network Packets*) del historial. Filas del historial de red: `netUp`, `netDown`, `pktUp`, `pktDown`.

### 3.9. Direcciones IP del servidor

El bloque «Direcciones IP del servidor» (*IP Addresses*) muestra `IPv4` e `IPv6`. Las direcciones externas se determinan mediante servicios de terceros (`api4.ipify.org`, `ipv4.icanhazip.com`, `v4.api.ipinfo.io/ip`, `ipv4.myexternalip.com/raw`, `4.ident.me`, `check-host.net/ip` para IPv4, y equivalentes para IPv6). La lista se recorre en orden hasta obtener la primera respuesta exitosa; el timeout por solicitud es de 3 s.

Particularidades:
- El resultado se **cachea** durante la vida del proceso: una dirección determinada con éxito no vuelve a solicitarse.
- Si ningún servicio responde, el campo queda como `N/A`. Para IPv6, en el primer `N/A` las solicitudes IPv6 se deshabilitan por completo para no perder tiempo en redes sin IPv6.
- Junto al bloque hay un botón de «ojo» para ocultar/mostrar las direcciones — descripción: «Ocultar o mostrar las direcciones IP del servidor» (*Toggle visibility of the IP*). Es solo un ocultamiento visual en la interfaz (por ejemplo, para capturas de pantalla); no afecta a las propias direcciones.

### 3.10. Conexiones TCP/UDP

El bloque «Estadísticas de conexiones» (*Connection Stats*) muestra el número total de conexiones TCP y UDP activas en el servidor (a nivel de todo el sistema, no solo Xray). El gráfico de historial es «Conexiones activas (TCP / UDP)» (*Active Connections*), filas `tcpCount`, `udpCount`.

### 3.11. Estado de Xray y control del proceso

La tarjeta «Xray» muestra el estado del proceso Xray-core y permite controlarlo.

#### Estados

| Valor | Etiqueta | Descripción | Cuándo se establece |
|---|---|---|---|
| `running` | «Ejecutándose» | *Running* | El proceso Xray está en ejecución. |
| `stop` | «Detenido» | *Stopped* | El proceso no está en ejecución y no hay ningún error de inicio registrado. |
| `error` | «Error» | *Error* | El proceso no está en ejecución, pero se registró un error de inicio. El texto del error se muestra en una ventana emergente con el título «Se produjo un error al ejecutar Xray» (*An error occurred while running Xray*). |
| — | «Desconocido» | *Unknown* | Se muestra mientras el estado aún no ha sido recibido. |

Junto al estado se muestra la **versión de Xray**.

#### Botones de control

- **Stop** (*Stop*). Llama a `POST /stopXrayService`. Si tiene éxito, el panel envía por WebSocket el nuevo estado `stop` y la notificación «Xray detenido correctamente» (*Xray service has been stopped*); en caso de error, el estado `error` con el texto del error. Importante: si el panel es accesible *a través del propio* Xray, detener Xray puede interrumpir la conexión con el panel — con conexión directa al panel no hay problema.
- **Restart** (*Restart*). Llama a `POST /restartXrayService`. Antes de la acción se muestra una confirmación «¿Reiniciar xray?» con la explicación «Recarga el servicio xray con la configuración guardada». Si tiene éxito — estado `running` y notificación «Xray reiniciado correctamente» (*Xray service has been restarted successfully*). El reinicio aplica la configuración guardada actual — úselo después de modificar la configuración.

> Nota. En este fork se ha añadido al dashboard un control completo Start / Stop / Restart para todos los tipos de autenticación; en la interfaz original de 3x-ui no hay un botón «iniciar» separado — el arranque se realiza mediante el reinicio.

#### Botón de visualización de logs de Xray

En la tarjeta Xray hay un botón para ver los logs de Xray (*Logs*). Aparece únicamente cuando en la configuración de Xray está configurado el log de acceso: el visor integrado lee exactamente ese archivo, por lo que sin log de acceso el botón está oculto. La visibilidad del botón está vinculada a un indicador separado `accessLogEnable` y ya no depende del límite de IP — la lista en línea y el límite de direcciones IP siguen funcionando incluso sin log de acceso (véase punto 8).

#### Selección de versión de Xray

La sección «Selección de versión» (*Version*) permite cambiar Xray-core a otra versión. La lista de versiones se carga mediante `GET /getXrayVersion`:

- La fuente es la API de GitHub del repositorio `XTLS/Xray-core` (`/releases`). Las solicitudes se cachean durante **15 minutos**; en caso de fallo de GitHub se devuelve la última lista obtenida con éxito para que el selector no quede vacío.
- En la lista solo aparecen versiones con formato `X.Y.Z` y **no anteriores al 26.4.25**.

Indicaciones: «Seleccione la versión a la que desea cambiar» (*Choose the version you want to switch to.*) y la advertencia «Importante: las versiones antiguas pueden no ser compatibles con la configuración actual» (*Choose carefully, as older versions may not be compatible with current configurations.*).

Cambio de versión: `POST /installXray/:version`. Escenario:

**Ejemplo.** Cambiar a una versión específica de Xray-core (la cookie de sesión debe haberse obtenido previamente mediante autenticación):

```bash
curl -X POST 'https://panel.example.com:2053/xpanel/installXray/v25.6.8' \
  -b cookie.txt
```

Aquí `v25.6.8` es la etiqueta de la lista que devuelve `GET /getXrayVersion`. La versión debe estar presente en esa lista; de lo contrario, el panel devolverá un rechazo.
1. La versión seleccionada se verifica en la lista actualizada de versiones (de lo contrario, se rechaza).
2. Xray se detiene.
3. Se descarga desde GitHub el archivo `Xray-<os>-<arch>.zip` para el SO y la arquitectura actuales (se admiten amd64/64, arm64-v8a, arm32-v7a/v6/v5, 386/32, s390x; para Windows — `xray.exe`). El tamaño del archivo y del binario está limitado a 200 MB.
4. El binario se reemplaza de forma atómica (mediante un archivo temporal + renombrado) y se marca como ejecutable.
5. Xray se reinicia.

Antes del cambio se muestra el diálogo «Cambiar versión de Xray» (*Do you really want to change the Xray version?*) con la descripción «Esto cambiará la versión de Xray a #version#». Si tiene éxito — notificación «Xray actualizado correctamente» (*Xray updated successfully*).

### 3.12. Actualización del panel (3X-UI)

Bloque de comprobación de actualizaciones del panel. Los datos llegan mediante `GET /getPanelUpdateInfo`:

| Campo | Descripción |
|---|---|
| Versión actual del panel | Versión del panel instalado. |
| Última versión del panel | Último lanzamiento de 3x-ui obtenido desde GitHub. |
| Actualización disponible | Indicador de que la última versión es más reciente que la actual. Si no se necesita actualización, se muestra «Panel actualizado» / «Actualizado». |

El botón **«Actualizar panel»** (*Update Panel*) lanza `POST /updatePanel`. Descripción: «Esto actualizará 3X-UI al último lanzamiento y reiniciará el servicio del panel». Antes de ejecutarse — confirmación «¿Realmente desea actualizar el panel?» con el texto «Esto actualizará 3X-UI a la versión #version# y reiniciará el servicio del panel».

Particularidades y limitaciones:
- La autoactualización es compatible **únicamente en Linux** (en otros SO se devuelve un error).
- El script de actualización se descarga desde el repositorio oficial (`raw.githubusercontent.com/MHSanaei/3x-ui/main/update.sh`, límite 2 MB) y se ejecuta mediante `bash`, aislado cuando sea posible a través de `systemd-run`.
- Si el lanzamiento es exitoso se muestra «La actualización del panel ha comenzado» (*Panel update started*); si la comprobación de actualización falla — «La comprobación de actualización del panel ha fallado». Durante la instalación se muestra la advertencia «Instalación en progreso. No actualice la página».

### 3.13. Actualización de archivos geográficos (GeoIP / GeoSite)

El botón/diálogo de actualización de bases geográficas llama a `POST /updateGeofile` (todos los archivos) o `POST /updateGeofile/:fileName` (un solo archivo). La actualización funciona con una lista blanca estricta de nombres y fuentes:

| Archivo | Fuente |
|---|---|
| `geoip.dat`, `geosite.dat` | `Loyalsoldier/v2ray-rules-dat` (latest) |
| `geoip_IR.dat`, `geosite_IR.dat` | `chocolate4u/Iran-v2ray-rules` (latest) |
| `geoip_RU.dat`, `geosite_RU.dat` | `runetfreedom/russia-v2ray-rules-dat` (latest) |

Comportamiento:
- El nombre del archivo se valida: se prohíben `..`, barras y rutas absolutas; solo se admiten `[a-zA-Z0-9._-]+.dat`. Los archivos que no están en la lista blanca no se descargan.
- Se utiliza la solicitud condicional `If-Modified-Since`: si el archivo no ha cambiado en el servidor de origen (HTTP 304), no se vuelve a descargar, solo se actualiza la marca de tiempo.
- Tras la descarga, Xray se **reinicia** (para que tome las nuevas bases).

**Ejemplo.** Actualizar solo las bases geográficas rusas sin tocar los demás archivos:

```bash
curl -X POST 'https://panel.example.com:2053/xpanel/updateGeofile/geoip_RU.dat' -b cookie.txt
curl -X POST 'https://panel.example.com:2053/xpanel/updateGeofile/geosite_RU.dat' -b cookie.txt
```

Para actualizar todos los archivos de la lista blanca de una vez — llame a `POST /updateGeofile` sin nombre de archivo.
- Diálogos: «¿Realmente desea actualizar el archivo geográfico?» con «Esto actualizará el archivo #filename#» para un solo archivo y «Esto actualizará todos los archivos geográficos» para el botón «Actualizar todos». Éxito — «Archivos geográficos actualizados correctamente».

### 3.14. Copia de seguridad y restauración de la base de datos

Bloque «Copia de seguridad y restauración» (*Backup & Restore*). El comportamiento depende del SGBD utilizado (SQLite por defecto o PostgreSQL).

#### Exportar base de datos (Copia de seguridad)

El botón «Exportar base de datos» / «Copia de seguridad» (*Back Up*) llama a `GET /getDb`. El archivo se entrega como adjunto:
- **SQLite**: primero se realiza un checkpoint (volcado del WAL) y luego se descarga el archivo `x-ui.db`. Descripción: «Haga clic para descargar el archivo .db que contiene la copia de seguridad de su base de datos actual…».
- **PostgreSQL**: se descarga un volcado `x-ui.dump` en formato personalizado (`pg_dump --format=custom --no-owner --no-privileges`). Las herramientas cliente de PostgreSQL deben estar instaladas en el servidor; de lo contrario se producirá un error sobre la ausencia de `pg_dump`.

#### Importar base de datos (Restauración)

El botón «Importar base de datos» / «Restauración» (*Restore*) carga el archivo mediante `POST /importDB` (campo de formulario `db`). Descripción: «Haga clic para seleccionar y cargar el archivo .db… para restaurar la base de datos desde la copia de seguridad».

Escenario para **SQLite**, seguro y con reversión:
1. El archivo se verifica en cuanto al formato SQLite y se guarda en un archivo temporal; luego se comprueba su integridad.
2. Xray se detiene, la BD actual se cierra y se renombra a `*.backup` (fallback).
3. El nuevo archivo ocupa el lugar de la BD activa, se realiza la inicialización y la migración. Si algo falla — se restaura el fallback.
4. Xray se reinicia.

Para **PostgreSQL** se carga el `.dump` (se verifica la firma `PGDMP`) y se aplica mediante `pg_restore --clean --if-exists --single-transaction …`. La descripción advierte explícitamente: «Esto reemplazará todos los datos actuales».

Mensajes: «Base de datos importada correctamente», «Se produjo un error al importar la base de datos», «…al leer la base de datos», «…al obtener la base de datos».

#### Archivo de migración (entre SQLite y PostgreSQL)

El botón «Descargar archivo de migración» (*Download Migration*) llama a `GET /getMigration` y genera una exportación portátil para ejecutar el panel sobre otro SGBD:
- En **SQLite** se descarga `x-ui.dump` (volcado SQL en texto plano).
- En **PostgreSQL** se descarga `x-ui.db` — una base SQLite lista, construida a partir de los datos de PostgreSQL.

### 3.15. Elementos adicionales de la interfaz

- **Indicador de clientes en línea.** El dashboard mantiene la fila `online` (*Online Clients* / «Clientes en línea») — número de clientes con una conexión activa. Se calcula cuando Xray está en ejecución (de lo contrario 0) y se registra en el historial con el mismo ciclo de 2 segundos. El gráfico se encuentra en la pestaña «Online».
- **Historial del sistema (gráficos).** Botón/sección «Gráficos» → «Historial del sistema» con pestañas: «Ancho de banda», «Paquetes», «Disco I/O», «Online», «Carga», «Conexiones», «Uso de disco». Los datos se obtienen mediante `GET /history/:metric/:bucket`; los intervalos de agregación permitidos (bucket, seg): **2, 30, 60, 180, 360, 720, 1440, 2880, 10080**, se reciben hasta 60 puntos por pestaña. En el propio selector de rango de la página hay botones **2m, 1h, 3h, 6h, 12h, 24h, 2d, 7d** (buckets `2, 60, 180, 360, 720, 1440, 2880, 10080` respectivamente). En los rangos largos **2d** y **7d** las etiquetas de tiempo en el eje incluyen la fecha en formato `MM-DD HH:MM`. El almacenamiento está organizado con un muestreo de tres niveles (rollup): los datos recientes se conservan con paso de 2 s durante la última **hora**, luego se promedian al paso de 1 min durante **48 horas** y al paso de 10 min durante **7 días**. Por eso los gráficos (CPU, RAM, tráfico, paquetes, conexiones, disco, online, carga) pueden consultarse durante un período **de hasta 7 días** (antes — hasta 48 horas), y cuanto más atrás en el tiempo, más gruesa es la granularidad. Métricas permitidas: `cpu, mem, swap, netUp, netDown, pktUp, pktDown, diskRead, diskWrite, diskUsage, tcpCount, udpCount, online, load1, load5, load15`. La etiqueta «Últimos 2 minutos» corresponde a bucket = 2 (modo tiempo real).

**Ejemplo.** Obtener la serie de carga de CPU de los últimos ~2 minutos (bucket = 2 s, hasta 60 puntos) y la misma serie agregada por 5 minutos (bucket = 300 s):

  ```bash
  curl 'https://panel.example.com:2053/xpanel/history/cpu/2' -b cookie.txt
  curl 'https://panel.example.com:2053/xpanel/history/cpu/300' -b cookie.txt
  ```

  La métrica puede sustituirse por cualquiera de las permitidas (`mem`, `netUp`, `tcpCount`, `load1`, etc.). Un bucket fuera de la lista blanca `2, 30, 60, 180, 360, 720, 1440, 2880, 10080` será rechazado.
- **Métricas de Xray** — bloque separado con el consumo de memoria y la recolección de basura de Xray (filas `xrAlloc, xrSys, xrHeapObjects, xrNumGC, xrPauseNs`) y el «Observatorio» (estado de las conexiones salientes). Solo funcionan si en la configuración de Xray se ha definido el bloque `metrics` (`listen 127.0.0.1:11111`, etiqueta `metrics_out`); de lo contrario se muestra «El endpoint de métricas de Xray no está configurado». En la ventana de métricas de Xray hay su propio selector de rango con botones **2m, 1h, 3h, 6h, 12h** (buckets `2, 60, 180, 360, 720`).

**Ejemplo** de bloque que activa el panel de métricas de Xray. En la sección de configuración de Xray deben estar presentes simultáneamente `metrics` (con etiqueta) y el inbound que escucha esa etiqueta:

  ```json
  {
    "metrics": {
      "tag": "metrics_out"
    },
    "inbounds": [
      {
        "listen": "127.0.0.1",
        "port": 11111,
        "protocol": "dokodemo-door",
        "settings": { "address": "127.0.0.1" },
        "tag": "metrics_out"
      }
    ]
  }
  ```

  La dirección `127.0.0.1:11111` no se expone al exterior intencionalmente — el panel la consulta localmente.
- **Selector de tema oscuro.** Se encuentra en el menú general / cabecera, no en el propio dashboard. Opciones: «Tema» (*Theme*) con las variantes «Oscuro» y «Ultra Oscuro» (*Ultra Dark*). Es únicamente una configuración visual del aspecto; no afecta al funcionamiento del panel.
- **Otros enlaces** en el entorno del dashboard (desde el menú / panel inferior): «Logs», «Configuración» — visualización del JSON final de Xray (`GET /getConfigJson`), «Documentación».

---

## 4. Inbounds: creación y parámetros generales

La sección **«Entrantes»** (inbounds) es la lista de todos los puntos de entrada de Xray a través de los cuales se conectan los clientes. Cada inbound almacena tanto campos «de panel» (nota, límite de tráfico, calendario de reinicio) como bloques JSON sin procesar de configuración de Xray (`settings`, `streamSettings`, `sniffing`).

La creación se realiza con el botón **«Crear conexión»** (*Add Inbound*), y la edición con **«Modificar conexión»** (*Modify Inbound*). Ambas operaciones se envían a los endpoints de API `POST /add` y `POST /update/:id`.

A continuación se describen todos los campos del formulario que **no** pertenecen a la configuración de un protocolo específico (clientes, cifrado, REALITY/TLS) y que **no** pertenecen al transporte/flujo (pestañas **«Flujo»**, **«Seguridad»**) — esos son temas de secciones separadas.

### 4.1. Campos generales del formulario

#### Remark (Nota)

| Parámetro | Valor |
|---|---|
| Campo | `remark` |
| Tipo | cadena |
| Por defecto | vacío |

Nombre legible por humanos del inbound, que se muestra en la lista y en los encabezados de los diálogos («¿Eliminar conexión "{remark}"?» etc.). La etiqueta del campo es **«Nota»**. No afecta al funcionamiento de Xray; sirve únicamente para facilitar la administración. Se recomienda asignar nombres únicos y descriptivos, ya que se usan en los nombres de los archivos exportados y en las confirmaciones de operaciones masivas.

#### Protocol (Protocolo)

| Parámetro | Valor |
|---|---|
| Campo | `protocol` |
| Etiqueta | **«Protocolo»** |
| Validación | `required,oneof=vmess vless trojan shadowsocks wireguard hysteria http mixed tunnel tun` |

Lista desplegable del protocolo del inbound. Valores permitidos:

| Valor | Nota |
|---|---|
| `vmess` | |
| `vless` | |
| `trojan` | |
| `shadowsocks` | |
| `wireguard` | |
| `hysteria` | Hysteria v2 es `hysteria` con `streamSettings.version = 2`; no existe un protocolo separado |
| `http` | |
| `mixed` | socks/http en un mismo puerto |
| `tunnel` | |
| `tun` | aceptado por el validador; no existe una constante de protocolo separada |

El campo es obligatorio (`required`). La elección del protocolo determina qué campos de configuración de clientes y qué transportes estarán disponibles (véanse las secciones específicas de cada protocolo).

> Importante: al guardar, el servicio normaliza `streamSettings`. Las configuraciones de transporte se conservan solo para los protocolos `vmess`, `vless`, `trojan`, `shadowsocks`, `hysteria`; para el resto (`http`, `mixed`, `tunnel`, `wireguard`, `tun`) el campo `streamSettings` se **borra forzosamente**.

Para inbounds de tipo `tunnel`/TProxy cuyo bloque `streamSettings` no contiene la clave `security` (variante sin transporte), el formulario se abre y guarda sin el error de validación `streamSettings.security Invalid input`.

#### Listen IP (IP de escucha)

| Parámetro | Valor |
|---|---|
| Campo | `listen` |
| Tipo | cadena |
| Por defecto | vacío → Xray escucha en `0.0.0.0` (todas las IPs) |

Dirección IP en la que el inbound acepta conexiones. Sugerencia del campo:

> «Dejar vacío para escuchar en todas las direcciones IP».

Al generar la configuración de Xray, el valor vacío se reemplaza por `0.0.0.0`. Además de una IP, el campo acepta una **ruta de Unix socket** — sugerencia:

> «También puede especificar una ruta de Unix socket (por ejemplo, /run/xray/in.sock) o un nombre de socket abstracto con el prefijo @ (por ejemplo, @xray/in.sock) para escuchar en un socket en lugar de un puerto TCP — en ese caso establezca el puerto en 0».

Así, el campo acepta dos formas de Unix socket: una ruta en el sistema de archivos (`/run/xray/in.sock`) y un nombre de socket abstracto con el prefijo `@` (`@xray/in.sock`). En ambos casos establezca `Port` en `0`.

Este campo se modifica cuando se necesita restringir el inbound a una sola interfaz (por ejemplo, `127.0.0.1` para un inbound que funcione únicamente como destino fallback detrás de Nginx) o cuando el inbound escucha en un Unix socket.

**Ejemplo.** Inbound que escucha solo en la interfaz local (destino fallback típico detrás de Nginx) y en un Unix socket:

```
listen = 127.0.0.1   puerto = 8443
listen = /run/xray/in.sock   puerto = 0
```

#### Port (Puerto)

| Parámetro | Valor |
|---|---|
| Campo | `port` |
| Etiqueta | **«Puerto»** |
| Validación | `gte=0,lte=65535` |
| Por defecto | — (lo establece el usuario) |

Puerto TCP/UDP de escucha. Se permiten valores de `0` a `65535`. El valor `0` se usa únicamente en combinación con la escucha en un Unix socket (véase arriba).

Al guardar, el servicio comprueba conflictos de puerto: dos inbounds no pueden ocupar simultáneamente el mismo `listen:port` para el mismo transporte (TCP/UDP). El transporte se determina a partir del protocolo y de `streamSettings`/`settings`: por ejemplo, `hysteria` y `wireguard` siempre ocupan UDP, `kcp`/`quic` — UDP, y la mayoría de los demás — TCP. En caso de conflicto, el guardado se rechaza con un error.

Adicionalmente, el panel no permite ocupar el **puerto reservado de la API interna de Xray** (etiqueta `api`, por defecto `62789` en `127.0.0.1`): un inbound TCP local cuya dirección de escucha coincide con ese puerto en loopback se rechaza con el mismo error de conflicto de puerto. El puerto real de la API se lee desde la plantilla de configuración de Xray (con valor de reserva `62789`). En los nodos (nodes) esta restricción no aplica — tienen su propio Xray.

> La etiqueta de Xray (`Tag`, única) se genera automáticamente a partir del puerto y el transporte con el formato `in-<puerto>-<tcp|udp|tcpudp|any>`; para un inbound desplegado en un nodo se añade el prefijo `n<nodeId>-`. En caso de colisión se añaden sufijos `-2`, `-3`, etc. Normalmente el usuario no edita la etiqueta.

#### Total traffic (Tráfico total, GB)

| Parámetro | Valor |
|---|---|
| Campo | `total` (en **bytes**) |
| Etiqueta | **«Uso total»** |
| Por defecto | `0` |

Límite de tráfico total del inbound. En el formulario el valor se introduce en gigabytes; en la base de datos se almacena en bytes. Sugerencia del campo:

> «= Sin límite. (unidad: GB)».

Es decir, **`0` significa sin límite**. Es un límite a nivel del inbound completo (no de clientes individuales); el tráfico efectivamente consumido se almacena en los campos `up` (enviado) y `down` (recibido) y se compara con `total`.

#### Expiry date / Duration (Fecha de vencimiento / duración)

| Parámetro | Valor |
|---|---|
| Campo | `expiryTime` (marca de tiempo Unix) |
| Etiqueta | **«Fecha de vencimiento»** (*Duration*) |
| Por defecto | vacío / `0` |

Período de validez del inbound. Sugerencia:

> «Dejar vacío para que sea indefinido».

El valor vacío (`0`) significa un inbound sin fecha de vencimiento. El valor se almacena como marca Unix; el formulario permite establecer tanto una fecha concreta como un período en días (cuenta relativa desde el momento actual — etiqueta en inglés del campo *Duration*).

#### Enabled (Habilitar)

| Parámetro | Valor |
|---|---|
| Campo | `enable` |
| Etiqueta | **«Habilitar»** (*Enabled*) |
| Por defecto | se establece al crear |

Indicador de actividad del inbound. Cambiar este indicador en la lista se procesa mediante un endpoint «ligero» separado `POST /setEnable/:id`, en lugar de una actualización completa — esto se hace a propósito para no reserializar todo el bloque `settings` (de todos los clientes) con cada clic en el interruptor de un inbound con miles de clientes. Al deshabilitar, el inbound se elimina del Xray en ejecución; al habilitar, se añade de nuevo.

#### Node / Deploy to (Nodo / Desplegar en)

| Parámetro | Valor |
|---|---|
| Campo | `nodeId` |
| Etiqueta | **«Desplegar en»**, **«Panel local»** |
| Por defecto | vacío (panel local) |

Selección de dónde funciona físicamente el inbound: en el panel local o en uno de los nodos registrados. Detalle de implementación: `nodeId = 0` se normaliza a `nil`, ya que `0` no es un id de nodo válido sino un artefacto del enlace del formulario; `nil`/`0` indica el panel local. Al guardar un inbound en un nodo desconectado, puede aparecer el mensaje «el cambio se sincronizará cuando el nodo se vuelva a conectar».

#### Estrategia de dirección para enlaces (Share address strategy)

| Parámetro | Valor |
|---|---|
| Campo | estrategia + (opcionalmente) dirección personalizada |
| Etiqueta | **«Estrategia de dirección para enlaces»** (*Share address strategy*) |
| Por defecto | **«Dirección de escucha del inbound»** (*Inbound listen*) |

La lista desplegable determina qué dirección se inserta en los **enlaces de compartición y códigos QR exportados** de este inbound. Valores:

| Valor | Etiqueta | Qué se inserta |
|---|---|---|
| `node` | **«Dirección del nodo»** (*Node address*) | dirección del nodo en el que funciona el inbound |
| `listen` | **«Dirección de escucha del inbound»** (*Inbound listen*) | dirección de escucha del propio inbound |
| `custom` | **«Personalizada»** (*Custom*) | dirección propia del campo **«Dirección de compartición personalizada»** (*Custom share address*) |

Al seleccionar **«Personalizada»** aparece el campo **«Dirección de compartición personalizada»**; en él se introduce el host o IP **sin esquema ni puerto** (el valor se valida). La opción **«Dirección del nodo»** solo aparece en la lista si existe un nodo habilitado en el que pueda funcionar este inbound; en caso contrario se oculta y el valor se ajusta a **«Dirección de escucha del inbound»**.

Esta estrategia afecta **únicamente** a los enlaces de compartición directos y los códigos QR. **No** afecta a la entrega de suscripciones — allí la dirección sigue determinándose por la lógica habitual del panel.

### 4.2. Sniffing (Análisis de tráfico)

La pestaña **«Sniffing»** edita el bloque `sniffing` de la configuración de Xray, que se almacena como JSON sin procesar. Sniffing permite a Xray «inspeccionar» el nombre de dominio real o el protocolo dentro de la conexión con fines de enrutamiento.

| Subcampo | Etiqueta | Propósito |
|---|---|---|
| `enabled` | (interruptor de pestaña) | Habilita/deshabilita el sniffing para el inbound |
| `destOverride` | — | Lista de protocolos para los que se intercepta la dirección de destino: `http`, `tls`, `quic`, `fakedns` |
| `metadataOnly` | **«Solo metadatos»** | Usar únicamente los metadatos de la conexión, sin leer la carga útil |
| `routeOnly` | **«Solo enrutamiento»** | Aplicar el resultado del sniffing solo para el enrutamiento, sin reescribir la dirección de destino |
| `domainsExcluded` | **«Dominios excluidos»** | Dominios excluidos del sniffing |
| (IPs excluidas) | **«IPs excluidas»** | Direcciones IP excluidas del sniffing |

- **`destOverride`** — conjunto de analizadores: `http` (detecta el dominio a partir del encabezado HTTP Host), `tls` (a partir del SNI), `quic` (a partir del ClientHello de QUIC), `fakedns` (coincidencia con el pool de FakeDNS). Normalmente se habilitan `http` y `tls` para detectar dominios.

**Ejemplo del bloque `sniffing`** (detectar dominio por HTTP y TLS, usar el resultado solo para enrutamiento sin tocar la red local):

```json
{
  "enabled": true,
  "destOverride": ["http", "tls"],
  "routeOnly": true,
  "domainsExcluded": ["courier.push.apple.com"]
}
```
- **`metadataOnly`** — cuando está habilitado, Xray no lee el contenido del primer paquete y se basa únicamente en los metadatos; es útil para no interferir con protocolos cuyos datos no pueden «inspeccionarse».
- **`routeOnly`** — el resultado del sniffing se usa únicamente por las reglas de enrutamiento; la dirección de la conexión en el outbound no se reescribe por el dominio detectado.

> Nota: el panel almacena `sniffing` como un bloque JSON opaco y no añade nada al guardarlo — todos los valores por defecto de estas casillas se forman en el lado de la aplicación cliente. El bloque puede editarse en formato sin procesar a través de la sección «JSON del entrante» (véase más abajo).

### 4.3. Allocate (estrategia de distribución de puertos)

El bloque `allocate` en `streamSettings` controla cómo Xray distribuye los puertos de escucha. Forma parte de la configuración de Xray; el panel lo almacena y transmite como parte de `streamSettings`/JSON del inbound. Parámetros (según la terminología de Xray-core):

| Subcampo | Propósito | Valores / por defecto |
|---|---|---|
| `strategy` | Estrategia de asignación de puertos | `always` — escuchar siempre en el puerto indicado (por defecto); `random` — cambiar periódicamente los puertos escuchados dentro del rango |
| `refresh` | Intervalo de cambio de puertos (minutos) con `random` | número entero de minutos (se recomienda 5; mínimo 2) |
| `concurrency` | Cuántos puertos mantener abiertos simultáneamente con `random` | entero (por defecto 3; no más de un tercio del ancho del rango de puertos) |

`strategy: always` mantiene el inbound en un solo puerto (modo estándar). `strategy: random` se usa en escenarios anti-bloqueo, cuando el inbound «salta» periódicamente por un rango de puertos; en ese caso `refresh` y `concurrency` cobran sentido. Estos valores solo deben modificarse cuando se usa deliberadamente el modo de puertos aleatorios.

**Ejemplo del bloque `allocate`** en `streamSettings` (modo de puertos aleatorios: mantener 3 puertos abiertos, rotar cada 5 minutos):

```json
{
  "allocate": {
    "strategy": "random",
    "refresh": 5,
    "concurrency": 3
  }
}
```

Para que esto funcione, el `port` del inbound debe especificarse como un rango (por ejemplo, `20000-20100`).

### 4.4. External Proxy (Proxy externo)

El campo **«External Proxy»** pertenece a la configuración de generación de enlaces de invitación y se almacena en `streamSettings` del inbound. Define una lista de direcciones externas alternativas (host/puerto, opcionalmente con TLS forzado — **«TLS forzado»**) que se insertan en los enlaces de cliente en lugar del `listen:port` real del inbound.

Se utiliza cuando los clientes deben conectarse no directamente al servidor sino a través de un proxy externo/reverse/CDN: en ese caso los enlaces compartidos contienen la dirección pública de ese frontend. No afecta al proceso de aceptación de conexiones de Xray — es una función «cosmética» de los enlaces generados. Campos del formulario relacionados: **«TLS forzado»**, **«Fingerprint»**, etiquetas de cada entrada.

### 4.5. Fallbacks (Fallbacks)

La sección **«Fallbacks»** define las reglas de redirección para las conexiones que no coinciden con ningún cliente del inbound. Está disponible para inbounds maestros con transporte TLS (VLESS/Trojan TCP-TLS). Se gestiona a través de los endpoints `GET /:id/fallbacks` / `POST /:id/fallbacks`.

Sugerencia de la sección:

> «Cuando una conexión en este inbound no coincide con ningún cliente, se redirige a otro lugar. Seleccione un inbound hijo a continuación para que los campos de enrutamiento (SNI / ALPN / Path / xver) se rellenen automáticamente a partir de su transporte, o deje la selección vacía y establezca Dest directamente (por ejemplo, 8080 o 127.0.0.1:8080) para redirigir a un servidor externo como Nginx. Cada inbound hijo debe escuchar en 127.0.0.1 con security=none».

La sección de fallbacks se muestra solo para inbounds VLESS/Trojan sobre RAW (TCP) con seguridad TLS o REALITY. Un nuevo inbound comienza con `security=none`, por lo que la sección puede parecer ausente inicialmente. En ese estado (VLESS/Trojan, RAW/TCP, seguridad aún no configurada), en lugar de la sección se muestra una sugerencia integrada: los fallbacks estarán disponibles después de seleccionar TLS o Reality en la pestaña **«Seguridad»**.

#### Campos de una fila de fallback

| Campo | Por defecto | Descripción |
|---|---|---|
| (inbound hijo) | — | Selección del inbound hijo (etiqueta **«Seleccionar inbound»**). Si se selecciona, los campos Name/Alpn/Path/Dest pueden rellenarse automáticamente a partir de su transporte |
| Name | vacío (= cualquiera) | Condición de coincidencia por nombre (SNI/nombre). Etiqueta para «cualquiera» — **«cualquiera»** |
| Alpn | vacío | Condición de coincidencia por ALPN |
| Path | vacío | Condición de coincidencia por ruta (para transportes WS/HTTP del inbound hijo) |
| Dest | auto | Hacia dónde redirigir. Marcador de posición **«auto (listen:puerto del hijo)»**. Se puede indicar un puerto (`8080`) o `host:puerto` (`127.0.0.1:8080`) |
| Xver | `0` | Versión del protocolo PROXY (**«Xver»**): `0` — deshabilitado, `1` o `2` — la versión correspondiente de PROXY protocol |
| (orden) | por posición | Orden de aplicación de las reglas; se establece con los botones **«Subir»**/**«Bajar»** |

Lógica de guardado: toda la lista de fallbacks del maestro se reemplaza de forma atómica. Una fila que no tiene ni inbound hijo seleccionado (`childId <= 0`) ni `Dest` definido se **omite**. Si el inbound hijo seleccionado coincide con el id del maestro, se anula. Al generar el JSON final: si `Dest` está vacío, se calcula a partir del inbound hijo como `listen:port`, donde `0.0.0.0`/`::`/`::0` se sustituyen por `127.0.0.1`; los campos vacíos `name`/`alpn`/`path` no se incluyen en el JSON de salida; `xver` se añade solo si es mayor que 0.

**Ejemplo del `settings.fallbacks` resultante** (el tráfico con `alpn=h2` va al destino WS en la ruta `/ws`; todo lo demás va al Nginx local en el puerto 8080):

```json
{
  "fallbacks": [
    { "alpn": "h2", "path": "/ws", "dest": "127.0.0.1:2001", "xver": 1 },
    { "dest": 8080 }
  ]
}
```

La última fila sin `name`/`alpn`/`path` es la regla «por defecto» que captura todo lo demás.

#### Botones y sugerencias de la sección de fallbacks

- **«Añadir fallback»** — añadir una fila; **«Aún no hay fallbacks»** — estado vacío.
- **«Añadir rápidamente todos los adecuados»** / **«Añadir todos»** — añade una fila de fallback para cada inbound adecuado que aún no esté conectado. Resultado: «Se añadieron {n} fallback(s)» o «No hay nuevos inbounds adecuados».
- **«Rellenar desde hijo»** — volver a obtener los campos de enrutamiento (SNI/ALPN/Path/xver) del transporte del inbound hijo seleccionado; tras ejecutar — «Rellenado desde hijo».
- **«Modificar campos de enrutamiento»** / **«Ocultar avanzado»** — mostrar/ocultar los campos detallados de la fila.
- Las etiquetas **«Enruta cuando»** y **«Por defecto — captura todo lo demás»** explican la condición de activación de cada fila.

Después de guardar los fallbacks, el servidor reinicia Xray para que los nuevos `settings.fallbacks` entren en vigor.

### 4.6. Reinicio periódico del tráfico

El bloque **«Reinicio de tráfico»** configura el reinicio automático de los contadores de tráfico del inbound según una programación. Descripción:

> «Reinicio automático del contador de tráfico a los intervalos indicados».

| Parámetro | Valor |
|---|---|
| Campo | `trafficReset` |
| Validación | `omitempty,oneof=never hourly daily weekly monthly` |
| Por defecto | `never` |
| Campo asociado | `lastTrafficResetTime` — marca del último reinicio (etiqueta **«Último reinicio»**) |

Lista desplegable:

| Valor | Etiqueta |
|---|---|
| `never` | **«Nunca»** |
| `hourly` | **«Cada hora»** |
| `daily` | **«Diariamente»** |
| `weekly` | **«Semanalmente»** |
| `monthly` | **«Mensualmente»** |

Para cada período hay un cron registrado que se ejecuta según el calendario correspondiente (`@hourly`, `@daily`, `@weekly`, `@monthly`). El cron selecciona todos los inbounds con el `trafficReset` indicado y para cada uno reinicia los contadores del propio inbound (`up=0`, `down=0`) **y** el tráfico de todos sus clientes. Es decir, el reinicio periódico afecta tanto al inbound como a sus clientes.

**Ejemplo del valor del campo.** Para que los contadores se restablezcan el primer día de cada mes, en el formulario se selecciona **«Mensualmente»**, lo que se guarda como:

```json
{ "trafficReset": "monthly" }
```

El valor `never` (por defecto) desactiva completamente el reinicio automático.

### 4.7. JSON del entrante (avanzado)

La sección **«Secciones JSON del entrante»** proporciona acceso directo a los bloques JSON sin procesar del inbound. Descripción:

> «JSON completo del entrante y editores individuales para settings, sniffing y streamSettings».

Editores disponibles:

| Pestaña | Etiqueta | Qué edita |
|---|---|---|
| **Todo** | «Objeto completo del entrante con todos los campos en un solo editor» | todo el objeto Inbound |
| **Configuración** | «Contenedor del bloque settings de Xray» | campo `settings` |
| **Sniffing** | «Contenedor del bloque sniffing de Xray» | campo `sniffing` |
| **Stream** | «Contenedor del bloque stream de Xray» | campo `streamSettings` |

Estos campos se serializan como objetos JSON anidados: los bloques vacíos se devuelven como `null`, y el texto que no es JSON válido se envuelve en una cadena para que los datos no se pierdan. Los errores de análisis al guardar se muestran con el prefijo **«JSON avanzado»**.

La ventana de visualización «JSON del entrante», al igual que la ventana de importación de inbound, utiliza un editor de código completo con resaltado de sintaxis JSON (en lugar de un campo de texto ordinario): la visualización de la configuración está en modo resaltado de solo lectura, y la importación en modo editable, lo que facilita la lectura y la edición.

### 4.8. Acciones sobre el inbound: QR / Edit / Reset / Delete y estadísticas

En la lista y en la tarjeta del inbound están disponibles las siguientes acciones (menú **«Menú»**):

#### Estadísticas de tráfico

Se muestra el tráfico agregado del inbound: **«Enviado/recibido»** (campos `up`/`down`), **«Tráfico total»**, **«Conexiones totales»**. En la tarjeta también aparecen **«Creado»**, **«Actualizado»**, **«Fecha de vencimiento»**.

En la lista de inbounds hay una columna **Speed** con la velocidad de tráfico actual de cada inbound (subida/bajada), calculada a partir de los incrementos de los contadores entre sondeos; la misma velocidad en tiempo real se muestra en la ventana de estadísticas del inbound. Cuando un sondeo no produce incremento, el valor de velocidad se restablece.

En el resumen de clientes de la página de inbounds, el estado se determina según la prioridad «agotado/finalizado»: los clientes cuyo período ha vencido o cuyo tráfico se ha agotado (y a los que la tarea automática ha quitado el `enable`) se clasifican como **«Agotado/Finalizado»** (*Depleted/Ended*) y no como el gris **«Deshabilitado»** (*Disabled*), sin contarlos dos veces. La clasificación coincide con la mostrada en la tarjeta del propio cliente y tiene en cuenta correctamente a los clientes vinculados a varios inbounds.

#### Código QR y copia de enlaces

- **«Más detalles»** — expande los enlaces de conexión y suscripción.
- Código QR del cliente: sugerencia **«Haga clic en el código QR para copiar»**.
- **«Copiar enlace»** (*Copy URL*), **«Exportar enlaces»**.

#### Edit (Modificar)

**«Modificar conexión»** — abre el formulario de edición (`POST /update/:id`). Al actualizar, el servicio vuelve a leer la fila existente, transfiere los campos modificados, regenera la etiqueta si es necesario (si la etiqueta anterior fue autogenerada) y sincroniza el runtime de Xray. Éxito — notificación **«Conexión actualizada correctamente»**.

#### Reset Traffic (Reiniciar tráfico)

**«Reiniciar tráfico»** — pone a cero los contadores `up`/`down` de este inbound (`POST /:id/resetTraffic`, establece `up=0, down=0`). Confirmación:

> «¿Reiniciar el tráfico de "{remark}"?» / «Restablece los contadores de envío/recepción de esta conexión a 0».

El reinicio del tráfico del inbound **no** afecta a los contadores de sus clientes (para ellos existen acciones separadas «Reiniciar tráfico de clientes»). Tras el reinicio se inicia un reinicio de Xray. Éxito — notificación **«Tráfico entrante reiniciado»**. También existe una variante masiva — **«Reiniciar tráfico de todas las conexiones»** (`POST /resetAllTraffics`).

#### Delete (Eliminar)

**«Eliminar conexión»** (`POST /del/:id`). Confirmación:

> «¿Eliminar la conexión "{remark}"?» / «La conexión y todos sus clientes serán eliminados. Esta acción no se puede deshacer».

La eliminación retira el inbound del Xray en ejecución (con reinicio si es necesario). Éxito — notificación **«Conexión eliminada correctamente»**. Eliminación masiva — `POST /bulkDel`, con informe por elemento y no más de un reinicio de Xray.

#### Otras acciones sobre los clientes del inbound

En el menú también están disponibles: **«Clonar»** (copia del inbound con nuevo puerto y lista de clientes vacía), **«Eliminar todos los clientes»** (`POST /:id/delAllClients` — elimina todos los clientes; el inbound en sí se conserva), **«Eliminar clientes deshabilitados»**, **«Vincular/Desvincular clientes»**, **«Importar»**/**«Exportar conexiones»** (`POST /import`). Los detalles de las operaciones con clientes corresponden a la sección sobre clientes.

---

## 5. Protocolos

Al crear un inbound, lo primero que se selecciona es el **Protocolo** («Protocol»). El protocolo determina qué método de autenticación y cifrado de tráfico aplicará Xray-core a ese inbound, qué conjunto de campos en `settings` habrá que rellenar, así como qué transportes (`network`) y tipos de seguridad (TLS / REALITY) están disponibles para él.

El campo de protocolo se establece una sola vez al crear el inbound y **no cambia al editarlo** (en el formulario de edición la lista desplegable está bloqueada). Para cambiar el protocolo es necesario crear un nuevo inbound.

### 5.1. Lista de protocolos admitidos

El servidor acepta el siguiente conjunto de valores para el campo `Protocol`:

```
oneof=vmess vless trojan shadowsocks wireguard hysteria http mixed tunnel tun mtproto
```

> A partir de la versión **3.3.0** se añadió el valor `mtproto` (proxy de Telegram) a la lista.

| Valor en la configuración | Propósito | Modelo de cliente |
|---|---|---|
| `vless` | Protocolo proxy principal (predeterminado al crear un inbound) | Clientes con UUID, soporte de flow y cifrado post-cuántico |
| `vmess` | Protocolo proxy clásico de Xray | Clientes con UUID y parámetro `security` |
| `trojan` | Proxy que se disfraza de HTTPS ordinario | Clientes con contraseña |
| `shadowsocks` | Proxy Shadowsocks (incluyendo SIP022 / 2022-blake3) | Un usuario o varios (2022) |
| `wireguard` | Inbound WireGuard | Peers (no clientes) |
| `hysteria` | Inbound Hysteria (versión 2 por defecto) | Clientes con token `auth` |
| `http` | Proxy HTTP clásico (forward proxy) | Cuentas user/pass, sin contabilidad de tráfico |
| `mixed` | Proxy combinado SOCKS + HTTP | Cuentas user/pass |
| `tunnel` | Reenviador transparente (xray `dokodemo-door`) | Sin clientes |
| `tun` | Interfaz TUN (solo renderizado de los existentes) | Sin clientes |
| `mtproto` | Proxy Telegram (MTProto), añadido en 3.3.0; lo gestiona un proceso separado `mtg`, no Xray | Sin clientes (acceso por secreto) |

> Nota sobre `tun`: el valor se mantiene en la lista por compatibilidad y para **mostrar** inbounds guardados previamente, pero en la versión actual del backend no se recomienda su creación — el soporte se considera obsoleto. No tiene sentido crear nuevos inbounds de este tipo.

> Nota sobre Hysteria 2: no existe un protocolo «hysteria2» separado. Es el protocolo `hysteria` con el campo `streamSettings.version = 2`. El esquema de enlace `hysteria2://` al generar enlaces de compartición se selecciona automáticamente cuando la versión del stream es 2.

No todos los protocolos admiten distribución por nodos (nodes). Solo se pueden desplegar en nodos: `vless`, `vmess`, `trojan`, `shadowsocks`, `hysteria`, `wireguard`. Los protocolos `http`, `mixed`, `tunnel`, `tun`, `mtproto` funcionan únicamente en el panel local.

### 5.2. Qué protocolos admiten TLS / REALITY / transporte

La posibilidad de habilitar una capa de seguridad u otra y el transporte depende del protocolo y de la red seleccionada (`streamSettings.network`):

| Capacidad | Disponible para protocolos | Redes permitidas (`network`) |
|---|---|---|
| **TLS** | `vmess`, `vless`, `trojan`, `shadowsocks` (además siempre para `hysteria`) | `tcp`, `ws`, `http`, `grpc`, `httpupgrade`, `xhttp` |
| **REALITY** | `vless`, `trojan` | `tcp`, `http`, `grpc`, `xhttp` |
| **flow (`xtls-rprx-vision`)** | solo `vless` | solo `tcp`, con `security = tls` o `reality` |
| **Stream / transporte** (pestaña «Flujo») | `vmess`, `vless`, `trojan`, `shadowsocks`, `hysteria` | — |

Para los protocolos `http`, `mixed`, `tunnel`, `tun`, `wireguard` la pestaña de transporte no está disponible — no tienen configuraciones de stream de Xray.

---

### 5.3. VLESS

Propósito: principal protocolo proxy moderno. Admite XTLS-Vision (`flow`), REALITY, así como cifrado post-cuántico a nivel del propio VLESS (campos `decryption` / `encryption`). Se usa por defecto para nuevos inbounds.

Campos del bloque `settings`:

| Campo | Valor por defecto | Descripción |
|---|---|---|
| `clients` | `[]` | Lista de clientes. Cada uno tiene: `id` (UUID), `email` (obligatorio), `flow`, límites (`limitIp`, `totalGB`, `expiryTime`), `enable`, `tgId`, `subId`, `comment`, `reset` |
| `decryption` | `none` | Parámetro de descifrado en el lado del servidor. Etiqueta en la UI: «Descifrado» (en inglés «Decryption») |
| `encryption` | `none` | Parámetro de cifrado complementario (se incluye en el enlace del cliente). Etiqueta: «Cifrado» (en inglés «Encryption») |
| `fallbacks` | `[]` | Lista de fallbacks (véase la sección sobre fallbacks); disponible cuando `network = tcp` y `security` = TLS o REALITY |
| `testseed` | (4 números: 900, 500, 900, 256) | «Vision testseed» — 4 enteros positivos para el relleno XTLS-Vision. Solo se aplica a clientes con flow `xtls-rprx-vision`; en caso contrario se ignora |

#### flow (`xtls-rprx-vision`)

`flow` se establece **en el cliente**, no en el inbound, y acepta uno de tres valores:

| Valor | Significado |
|---|---|
| `` (vacío) | Sin XTLS-flow (por defecto) |
| `xtls-rprx-vision` | XTLS-Vision — modo recomendado para VLESS sobre TCP+TLS/REALITY |
| `xtls-rprx-vision-udp443` | El mismo Vision, pero con gestión de UDP/443 (QUIC) |

El campo `flow` solo está disponible para selección cuando se cumplen todas las condiciones: protocolo `vless`, `network = tcp` y `security` = `tls` o `reality`. El campo **Vision testseed** en el formulario solo se muestra bajo las mismas condiciones.

> Excepción para XHTTP: con VLESS sobre `network = xhttp` con autenticación post-cuántica VLESS habilitada (`encryption`/`decryption`, vlessenc), el flow `xtls-rprx-vision` también es válido — independientemente de la capa de seguridad, incluido REALITY. En este caso el panel transmite correctamente `xtls-rprx-vision` en los enlaces de compartición y en las suscripciones (incluido el formato Clash/Mihomo), de modo que el cliente recibe la configuración precisamente con Vision.

#### Descifrado / Cifrado (autenticación post-cuántica VLESS)

Los campos `decryption` y `encryption` son autenticación a nivel del propio VLESS (separada del TLS/REALITY de transporte). Por defecto ambos son `none`. En el formulario debajo de estos campos hay un bloque de **«Generación de claves»** — una lista desplegable de modo y un botón **«Generar»** (junto a él, un botón **«Limpiar»**). La lista desplegable contiene seis opciones: **X25519 (native)**, **X25519 (xorpub)**, **X25519 (random)**, **ML-KEM-768 (native)**, **ML-KEM-768 (xorpub)**, **ML-KEM-768 (random)** — es decir, dos tipos de clave (el clásico X25519 y el post-cuántico ML-KEM-768), cada uno en tres modos:

- **native** — par de claves base del tipo seleccionado;
- **xorpub** — modo derivado con procesamiento adicional de la parte pública;
- **random** — modo derivado con componente aleatorio.

Seleccione el modo deseado en la lista y pulse **«Generar»**: el panel rellenará **ambos** campos (`decryption` y `encryption`) con el par de valores listo para ese modo. El botón **«Limpiar»** restablece ambos campos a `none`.

Debajo del bloque se muestra una línea de estado **«Seleccionado: …»** que reconoce, a partir de la cadena generada, tanto el tipo de clave (X25519 o ML-KEM-768) como el modo (native / xorpub / random) y los muestra. Los campos vacíos o `none` se muestran como «None».

Técnicamente los botones llaman a `GET /panel/api/server/getNewVlessEnc` (generación de claves mediante `xray vlessenc`) y rellenan **ambos** campos con valores emparejados del tipo `mlkem768x25519plus.native.<rtt>.<role>` (por ejemplo, `decryption = mlkem768x25519plus.native.600s.server-x25519`, `encryption = mlkem768x25519plus.native.0rtt.client-x25519`). El parámetro `decryption` permanece en el servidor; `encryption` va al enlace del cliente.

> Importante: al generar la configuración del inbound para Xray, el panel elimina lo superfluo: si en `settings` queda `encryption` (que pertenece al lado del cliente), se **extrae** de la configuración del servidor. En el servidor solo queda `decryption`.

Cuándo elegir VLESS: es la opción recomendada por defecto para un nuevo inbound, especialmente en combinación con REALITY (sin certificado propio) o con TLS + XTLS-Vision.

**Ejemplo: bloque `settings` de un inbound VLESS con un cliente y XTLS-Vision.** El campo `flow` está en el cliente; `decryption` permanece en el servidor:

```json
{
  "clients": [
    {
      "id": "d342d11e-d424-4583-b36e-524ab1f0afa4",
      "email": "user1",
      "flow": "xtls-rprx-vision",
      "limitIp": 2,
      "totalGB": 0,
      "expiryTime": 0,
      "enable": true
    }
  ],
  "decryption": "none"
}
```

Para la combinación REALITY, el bloque `streamSettings` correspondiente (pestaña «Transport» → Security: REALITY) tiene este aspecto:

```json
{
  "network": "tcp",
  "security": "reality",
  "realitySettings": {
    "dest": "www.microsoft.com:443",
    "serverNames": ["www.microsoft.com"],
    "privateKey": "<clave privada X25519>",
    "shortIds": ["", "6ba85179e30d4fc2"]
  }
}
```

---

### 5.4. VMess

Propósito: protocolo proxy clásico de Xray. Autenticación por UUID; en el cliente se configura adicionalmente el método de cifrado de la carga útil (`security`).

Campos del bloque `settings`:

| Campo | Valor por defecto | Descripción |
|---|---|---|
| `clients` | `[]` | Lista de clientes |

Cada cliente VMess (además de los campos comunes `email`, límites, `enable`, `tgId`, `subId`, `comment`, `reset`) tiene:

| Campo del cliente | Valor por defecto | Descripción |
|---|---|---|
| `id` | — | UUID del cliente |
| `security` | `auto` | Método de cifrado de la carga útil VMess. Valores permitidos: `aes-128-gcm`, `chacha20-poly1305`, `auto`, `none`, `zero` |

Valores de `security`:
- `auto` — Xray elige el cifrado según la plataforma (recomendado);
- `aes-128-gcm`, `chacha20-poly1305` — cifrados AEAD fijos;
- `none` — sin cifrado de la carga útil (solo tiene sentido sobre TLS);
- `zero` — sin cifrado ni autenticación de la carga útil.

> Compatibilidad histórica: los registros antiguos podían almacenar `security: ""` — al leerlos la cadena vacía se convierte en `auto`. Al generar la configuración del servidor, el campo `security` de los clientes VMess **se elimina** de `settings`, ya que para el inbound no es necesario.

Cuándo elegir VMess: para compatibilidad con clientes antiguos o configuraciones existentes. Para nuevos despliegues generalmente es preferible VLESS.

---

### 5.5. Trojan

Propósito: proxy que imita tráfico HTTPS ordinario. Autenticación por contraseña. Al igual que VLESS, admite fallbacks y (con `network = tcp`) REALITY/TLS.

Campos del bloque `settings`:

| Campo | Valor por defecto | Descripción |
|---|---|---|
| `clients` | `[]` | Lista de clientes |
| `fallbacks` | `[]` | Lista de fallbacks (disponible con `network = tcp` y TLS/REALITY) |

El campo clave de cada cliente Trojan:

| Campo del cliente | Valor por defecto | Descripción |
|---|---|---|
| `password` | — | Contraseña del cliente (obligatoria, mínimo 1 carácter) |
| `email` | — | Identificador único del cliente |

El resto de los campos del cliente son comunes (`limitIp`, `totalGB`, `expiryTime`, `enable`, `tgId`, `subId`, `comment`, `reset`).

Cuándo elegir Trojan: cuando se necesita disfraz de HTTPS en el puerto 443, incluyendo fallbacks a un servidor web (Nginx) para conexiones no autorizadas.

**Ejemplo: bloque `settings` de Trojan con fallback a un servidor web local.** Las conexiones no autorizadas (sin contraseña válida) se redirigen a Nginx, que escucha en `127.0.0.1:8080`:

```json
{
  "clients": [
    { "password": "S3cret-Pass-1", "email": "user1" }
  ],
  "fallbacks": [
    { "dest": "127.0.0.1:8080" }
  ]
}
```

Para el fallback se necesita `network = tcp` y Security = TLS o REALITY; de lo contrario el campo fallbacks no está disponible.

---

### 5.6. Shadowsocks

Propósito: proxy Shadowsocks ligero. Admite tanto los cifrados AEAD obsoletos como los nuevos métodos SIP022 (`2022-blake3-*`). Puede funcionar en modo monousuario o multiusuario.

Campos del bloque `settings`:

| Campo | Valor por defecto | Descripción |
|---|---|---|
| `method` | `2022-blake3-aes-256-gcm` | Método de cifrado del inbound. Etiqueta en la UI: «Método de cifrado» (en inglés «Encryption method») |
| `password` | `` | Contraseña del inbound (para los métodos 2022 se genera automáticamente según el método seleccionado) |
| `network` | `tcp,udp` | Transporte. Etiqueta: «Red» (en inglés «Network»). Opciones: `tcp,udp` (TCP, UDP), `tcp`, `udp` |
| `clients` | `[]` | Lista de clientes |
| `ivCheck` | `false` (desact.) | Interruptor «ivCheck» — protección contra reutilización de IV |

#### Métodos de cifrado (`method`)

Conjunto permitido:

| Método | Categoría |
|---|---|
| `aes-256-gcm` | AEAD obsoleto |
| `chacha20-poly1305` | AEAD obsoleto |
| `chacha20-ietf-poly1305` | AEAD obsoleto |
| `xchacha20-ietf-poly1305` | AEAD obsoleto |
| `2022-blake3-aes-128-gcm` | SS 2022 (recomendado) |
| `2022-blake3-aes-256-gcm` | SS 2022 (por defecto) |
| `2022-blake3-chacha20-poly1305` | SS 2022, monousuario |

Lógica del panel según los métodos:
- **Métodos 2022** (`2022-blake3-*`) se consideran «SS 2022». El método `2022-blake3-chacha20-poly1305` es **monousuario** (no admite multiusuario); los demás métodos 2022 permiten varios clientes. El campo de contraseña (con botón de generación que ajusta la longitud de la clave al método) se muestra en el formulario precisamente para los métodos 2022.
- **Cifrados obsoletos** (`aes-*`, `chacha20-*`) funcionan con el esquema clásico «un método + una contraseña».

> Normalización antes de ejecutar Xray: para los cifrados obsoletos cada cliente debe llevar el `method` coincidente con el del inbound (de lo contrario Xray falla con «unsupported cipher method:»). Para los métodos 2022 es al contrario — el campo `method` del cliente debe estar **vacío** (de lo contrario Xray rechaza el inbound con «users must have empty method»). El panel normaliza los datos automáticamente al cambiar el método.

> Regeneración de claves de cliente al cambiar el tamaño de clave: para Shadowsocks-2022, al cambiar el método de cifrado a uno con distinto tamaño de clave (por ejemplo entre `2022-blake3-aes-256-gcm` y `2022-blake3-aes-128-gcm`), el panel regenera automáticamente las PSK de los clientes con la nueva longitud al guardar el inbound. De lo contrario las claves antiguas mantendrían su longitud anterior y Xray las rechazaría. Consecuencia: los clientes afectados necesitan obtener de nuevo la suscripción — los enlaces anteriores dejarán de funcionar.

Cuándo elegir Shadowsocks: para despliegues sencillos sin enmascaramiento TLS; la elección moderna son los métodos 2022-blake3.

**Ejemplo: bloque `settings` de Shadowsocks para el método 2022-blake3 (modo multiusuario).** El inbound tiene su propia contraseña (clave base64 de la longitud necesaria); cada cliente tiene la suya; el campo `method` del cliente está **vacío**:

```json
{
  "method": "2022-blake3-aes-256-gcm",
  "password": "d2hhdGV2ZXItMzItYnl0ZS1iYXNlNjQta2V5LWhlcmU=",
  "network": "tcp,udp",
  "clients": [
    {
      "email": "user1",
      "password": "Y2xpZW50LWtleS0zMi1ieXRlcy1iYXNlNjQtaGVyZQ==",
      "method": ""
    }
  ]
}
```

Para los cifrados legacy (`aes-256-gcm` etc.) es al revés: una sola contraseña para el inbound, y el `method` del cliente debe coincidir con el del inbound.

---

### 5.7. Dokodemo-door / Tunnel (reenviador transparente)

Propósito: reenviador transparente (en el panel — protocolo `tunnel`, que implementa el comportamiento de `dokodemo-door`). Acepta tráfico y lo redirige a la dirección/puerto indicados, sin autenticación ni clientes.

Campos del bloque `settings`:

| Campo | Valor por defecto | Descripción |
|---|---|---|
| `rewriteAddress` | (ninguno) | «Reescribir dirección» (en inglés «Rewrite address») — dirección de destino a la que se redirige el tráfico |
| `rewritePort` | (ninguno) | «Reescribir puerto» (en inglés «Rewrite port») — puerto de destino (0–65535) |
| `allowedNetwork` | `tcp,udp` | «Red permitida» (en inglés «Allowed network»). Opciones: `tcp,udp`, `tcp`, `udp` |
| `portMap` | `{}` | «Mapeo de puertos» — mapa puerto→puerto (Record<string,string>) |
| `followRedirect` | `false` (desact.) | «Seguir redirect» (en inglés «Follow redirect») — usar la dirección de destino original de la conexión interceptada |

> Pestaña «Transport» para Tunnel: el inbound de este tipo tiene disponible la pestaña **«Transport»**, limitada a la configuración de `sockopt` — esto es suficiente para el modo **TProxy** (proxy transparente/redirect mediante `sockopt.tproxy`). La lista desplegable de selección de transporte (`network`) y la pestaña «Security» para Tunnel están ocultas, ya que TLS/REALITY no son compatibles con este tipo.

Cuándo elegir: para proxy transparente/redirección de puertos a servicios internos.

El campo «Reescribir puerto» (`rewritePort`) se puede dejar vacío: al borrar el valor este simplemente se excluye de la configuración del inbound, sin provocar un error de guardado. (Anteriormente vaciar este campo producía un error de validación en `settings.rewritePort` y bloqueaba el guardado, incluso a través de la pestaña JSON.)

---

### 5.8. SOCKS / HTTP (protocolo `mixed`)

En esta versión no existe un protocolo `socks` separado — SOCKS y el proxy HTTP están unidos en el protocolo **`mixed`** (SOCKS + HTTP combinados). Además existe un proxy `http` puro independiente.

#### 5.8.1. Mixed (SOCKS + HTTP)

Campos del bloque `settings`:

| Campo | Valor por defecto | Descripción |
|---|---|---|
| `auth` | `password` | «Auth» — modo de autenticación. Opciones: `password` (por usuario/contraseña) o `noauth` (sin autorización) |
| `accounts` | (opcional) | «Cuentas» — lista de pares user/pass. Con `auth = noauth` el campo no se escribe en la configuración |
| `udp` | `false` (desact.) | Interruptor «UDP» — soporte de UDP a través de SOCKS |
| `ip` | `127.0.0.1` | «UDP IP» — dirección local para asociaciones UDP. El campo se muestra solo cuando `udp` está habilitado |

Las cuentas se añaden con el botón «Agregar»; al añadir se generan un usuario aleatorio (8 caracteres) y una contraseña aleatoria (12 caracteres), que se pueden editar.

#### 5.8.2. HTTP (proxy puro)

Propósito: proxy HTTP de reenvío clásico. A nivel de Xray no rastrea clientes como «facturables» (sin email/límites) — solo hay una lista de cuentas.

Campos del bloque `settings`:

| Campo | Valor por defecto | Descripción |
|---|---|---|
| `accounts` | `[]` | «Cuentas» — lista de pares user/pass (ambos campos son obligatorios) |
| `allowTransparent` | `false` (desact.) | «Permitir transparente» (en inglés «Allow transparent») — reenviar solicitudes con el encabezado Host original |

Cuándo elegir SOCKS/HTTP: para acceso proxy local o de servicio sin enmascaramiento complejo. `mixed` es conveniente porque un solo puerto atiende tanto a clientes SOCKS como HTTP.

---

### 5.9. WireGuard (inbound)

Propósito: inbound WireGuard. A diferencia de los protocolos proxy, no opera con «clientes» — en su lugar se configuran **peers** (dispositivos que el servidor acepta). El transporte y TLS/REALITY no son aplicables.

Campos del bloque `settings`:

| Campo | Valor por defecto | Descripción |
|---|---|---|
| `secretKey` | — | Clave privada del servidor (obligatoria). Junto a ella hay un botón de generación; la clave pública se muestra automáticamente (campo de solo lectura) |
| `mtu` | (opcional) | MTU de la interfaz |
| `noKernelTun` | `false` (desact.) | «TUN sin kernel» (en inglés «No-kernel TUN») — usar TUN en espacio de usuario en lugar del del kernel |
| `domainStrategy` | (opcional) | «Domain Strategy» — estrategia de resolución de dominios: `ForceIP`, `ForceIPv4`, `ForceIPv4v6`, `ForceIPv6`, `ForceIPv6v4` |
| `peers` | `[]` | Lista de peers |

Campos de cada peer:

| Campo del peer | Valor por defecto | Descripción |
|---|---|---|
| `privateKey` | (opcional) | Clave privada del cliente — se almacena para que el panel pueda mostrar la configuración al usuario (solo en peers de inbound) |
| `publicKey` | — | Clave pública del peer (obligatoria) |
| `preSharedKey` (PSK) | (opcional) | Clave compartida adicional |
| `allowedIPs` | `[]` | IPs permitidas. Al añadir un nuevo peer el panel propone automáticamente la siguiente dirección libre (por defecto `10.0.0.2/32`) |
| `keepAlive` | (opcional) | «Keep-alive» — intervalo de mantenimiento de la conexión |
| `comment` | (opcional) | «Comment» — etiqueta libre del peer; se muestra junto al encabezado «Peer N» y se incluye en el enlace de compartición y en el `remark` del archivo `.conf` |

El botón «Agregar peer» genera un nuevo par de claves y asigna el siguiente `allowedIPs`. Cada peer puede eliminarse (para el único restante la eliminación no está disponible).

El campo «Comment» del peer ayuda a distinguir dispositivos: su texto se muestra en el formulario junto al encabezado «Peer N», y también aparece en el enlace de compartición y en el `remark` del archivo `.conf` generado, de modo que el dispositivo es fácil de identificar en la aplicación cliente. Este campo es del panel — xray-core ignora los campos desconocidos del peer.

#### Domain Strategy y pestaña Transport

Además de los peers, el inbound WireGuard tiene el campo **Domain Strategy** (estrategia de resolución de dominios: `ForceIP`, `ForceIPv4`, `ForceIPv4v6`, `ForceIPv6`, `ForceIPv6v4`). El campo es opcional y se escribe en la configuración solo si está definido.

> El campo **Workers** (`workers`, número de hilos de trabajo) se eliminó de los formularios de WireGuard (tanto inbound como outbound): a partir de xray-core v26.6.22 el motor ya no lo utiliza y se basa en el mecanismo interno de wireguard-go. Las configuraciones guardadas anteriormente funcionan sin cambios — al procesarlas el campo simplemente se descarta; no se necesita migración.

Para WireGuard también está disponible la pestaña **«Transport»** — pero en versión reducida: en ella solo se configuran `sockopt` y la ofuscación **Finalmask**. La lista desplegable de selección de transporte (`network`) está oculta, ya que WireGuard siempre escucha por UDP. En los registros de ruido (noise), Finalmask tiene un campo separado **Rand Range** (rango de bytes 0–255, con validación), y como método de ofuscación para WireGuard e Hysteria está disponible **Salamander**.

Cuándo elegir WireGuard: cuando se necesita precisamente un túnel VPN WireGuard, no un proxy enmascarado.

---

### 5.10. Hysteria (v2 por defecto)

Propósito: inbound Hysteria sobre QUIC. El panel funciona por defecto con la versión 2. Cada cliente se autentica con un token `auth` en lugar de UUID/contraseña. TLS para Hysteria está siempre disponible (véase la tabla de capacidades en 5.2).

Campos del bloque `settings`:

| Campo | Valor por defecto | Descripción |
|---|---|---|
| `version` | `2` | Versión del protocolo (mínimo 1; el panel usa 2 por defecto) |
| `clients` | `[]` | Lista de clientes |

El campo clave de cada cliente es `auth` (token, obligatorio) más los campos comunes (`email`, límites, `enable`, `tgId`, `subId`, `comment`, `reset`).

Los parámetros adicionales se configuran en `streamSettings.hysteriaSettings`:

| Campo | Valor / opciones | Descripción |
|---|---|---|
| `version` | fijado en 2 (campo bloqueado) | «Versión» (en inglés «Version») |
| `udpIdleTimeout` | (entero ≥ 1, seg.) | «UDP idle timeout (s)» — tiempo de inactividad UDP |
| `masquerade` | desactivado por defecto | «Masquerade» — disfraz de servidor web ordinario ante solicitudes «no autorizadas» |

Al activar `masquerade` se puede seleccionar el tipo (`type`):
- `` — default (página 404);
- `proxy` — proxy inverso (campos «Upstream URL», «Reescribir Host», «Omitir TLS verify»);
- `file` — servir directorio (campo «Directorio», por ejemplo `/var/www/html`);
- `string` — respuesta fija (campos «Código de estado», «Body», «Encabezados»).

Cuándo elegir Hysteria: cuando se necesita transporte QUIC y resistencia en canales inestables/móviles; el enmascaramiento aumenta la discreción del punto de entrada.

---

### 5.11. MTProto (proxy para Telegram)

> Añadido en la versión **3.3.0**. Valor del protocolo — `mtproto`.

MTProto es el protocolo del proxy propio de Telegram. En 3X-UI este inbound **no lo gestiona Xray sino un proceso separado `mtg`**, que controla el propio panel. El panel compara periódicamente los inbounds MTProto habilitados con los procesos `mtg` en ejecución: levanta los que faltan, detiene los sobrantes y recoge los contadores de tráfico de las métricas de `mtg`. Por eso la **contabilidad de tráfico** de este inbound funciona como en los protocolos habituales.

Mensaje de ayuda oficial en el formulario:

> «MTProto es gestionado por un proceso mtg separado, no por Xray. Los ajustes de transporte y los clientes no se aplican aquí — comparte el enlace de abajo en Telegram.»

Consecuencias:

- Las pestañas **«Transport» (Stream Settings) y «Clientes» no se aplican a este inbound** — el acceso se define por un único secreto, no por una lista de clientes.
- El inbound MTProto se ejecuta **solo en el panel principal**; no se despliega en nodos hijos (nodes) (se omiten los inbounds con `NodeID` definido).

- La pestaña **«Sniffing»** para MTProto está oculta — este protocolo lo gestiona el proceso `mtg`, no Xray, por lo que el sniffing no es aplicable.

**Campos.** Se almacenan en `settings` del inbound:

| Campo en la UI | Clave | Descripción |
|---|---|---|
| Remark | `remark` | Etiqueta del inbound. |
| Listen IP | `listen` | IP de escucha (vacío = todas las interfaces). |
| Port | `port` | Puerto del proxy. |
| Secreto | `settings.secret` | Secreto de acceso en formato **FakeTLS**. |
| Dominio de cobertura (FakeTLS) | `settings.fakeTlsDomain` | Dominio cuyo tráfico HTTPS imita el proxy. |

**Formato del secreto (FakeTLS).** El panel construye el secreto automáticamente en el formato correcto: resultado = `ee` + 32 caracteres hex + código hex del dominio de cobertura, es decir `ee<hex32><hex(fakeTlsDomain)>`. El prefijo `ee` activa el modo FakeTLS, y el dominio (por ejemplo, un sitio conocido) sirve para disfrazar el tráfico de HTTPS ordinario. Basta con indicar el dominio — el panel construirá el resto automáticamente.

#### Domain-fronting y opciones avanzadas de mtg

El inbound MTProto tiene parámetros adicionales del proceso `mtg`. Los campos **Domain fronting IP**, **Domain fronting port** y **Domain fronting PROXY protocol** indican adónde envía `mtg` el tráfico no-Telegram (por ejemplo, a un sitio falso de NGINX): si se deja el IP vacío, se usa el dominio FakeTLS mediante DNS; el puerto por defecto es `443`. Adicionalmente están disponibles **Accept PROXY protocol** (para el listener), **IP preference** (`prefer-ipv6` / `prefer-ipv4` / `only-ipv6` / `only-ipv4`) y **Debug logging**. Cada valor se escribe en el archivo `mtg-<id>.toml` solo si está definido.

#### Enrutamiento del tráfico de Telegram a través de Xray

El interruptor **«Route through Xray»** (desactivado por defecto) y el campo opcional **Outbound** permiten subordinar el egress de Telegram al enrutador de Xray. Al activarlo el panel inserta en la configuración de Xray un puente SOCKS local con la etiqueta del propio inbound, y `mtg` envía el tráfico de Telegram a través de él. Después el tráfico se puede emparejar con reglas en la pestaña «Routing» o dirigir forzosamente al outbound o balanceador seleccionado mediante el campo **Outbound** (si el campo está vacío, deciden las reglas de enrutamiento).

**Cómo distribuirlo al usuario.** Para el inbound MTProto el panel genera un enlace de invitación:

**Ejemplo: secreto FakeTLS y enlace listo.** Si el dominio de cobertura es `www.cloudflare.com`, el secreto se construye como `ee` + 32 caracteres hex + código hex del dominio, por ejemplo:

```
secret = ee1a2b3c4d5e6f70819293a4b5c6d7e8f7777772e636c6f7564666c6172652e636f6d
```

Enlace de invitación listo (este enlace y el código QR se envían al usuario en Telegram):

```
tg://proxy?server=203.0.113.10&port=443&secret=ee1a2b3c4d5e6f70819293a4b5c6d7e8f7777772e636c6f7564666c6172652e636f6d
```

```
tg://proxy?server=<dirección>&port=<puerto>&secret=<secreto>
```

(equivalente — `https://t.me/proxy?server=…&port=…&secret=…`). Este enlace y el código QR deben enviarse al usuario de Telegram — al abrirlo el proxy se añade directamente a la aplicación. El enlace también se entrega a través del servidor de suscripciones.

**Cuándo usar.** Método estándar para eludir los bloqueos de Telegram; el enmascaramiento FakeTLS (dominio de cobertura) hace que el tráfico parezca una visita ordinaria al sitio indicado.

### 5.12. Guía rápida para elegir protocolo

- **VLESS** — elección por defecto; la mejor opción con REALITY o TLS + XTLS-Vision, admite autenticación post-cuántica.
- **Trojan** — disfraz de HTTPS con fallbacks a servidor web.
- **VMess** — compatibilidad con clientes antiguos.
- **Shadowsocks** — proxy sencillo sin TLS; la elección moderna son los métodos `2022-blake3-*`.
- **Hysteria** — QUIC, resistencia en canales deficientes.
- **mixed / http** — proxies SOCKS/HTTP de servicio.
- **WireGuard** — túnel VPN completo.
- **tunnel** — redirección transparente de puertos.
- **MTProto** — proxy para eludir bloqueos de Telegram (FakeTLS); proceso separado `mtg`.

---

## 6. Transporte (Stream Settings)

El transporte (en la interfaz del panel — campo **«Transporte»**, en inglés *Transmission*) determina la forma en que Xray-core transfiere datos dentro de un inbound: qué protocolo de red se utiliza sobre TLS/Reality y cómo se encuadra exactamente el tráfico. Estos parámetros se guardan en el objeto `streamSettings` de la configuración de Xray y se configuran en la pestaña de transporte del editor de inbound. El cifrado (TLS / Reality) se trata en una sección aparte — aquí solo se describe la selección de red y sus parámetros.

### 6.1. Selección de la red de transmisión

La red se selecciona en el desplegable **«Transporte»** (`streamSettings.network`). El valor predeterminado es `tcp` (que aparece en la lista como **RAW**). Las opciones disponibles son:

| Valor en la lista | Campo `network` | Transporte |
| --- | --- | --- |
| RAW | `tcp` | TCP estándar (renombrado a RAW en versiones recientes de Xray), opcionalmente con ofuscación HTTP |
| mKCP | `kcp` | Transporte UDP fiable mKCP |
| WebSocket | `ws` | WebSocket sobre HTTP(S) |
| gRPC | `grpc` | Túnel gRPC (HTTP/2) |
| HTTPUpgrade | `httpupgrade` | HTTP Upgrade |
| XHTTP | `xhttp` | XHTTP / SplitHTTP — transporte multiplexado moderno |

Al cambiar el valor, el panel borra el bloque de configuración de la red anterior y rellena el bloque de la nueva red con los valores predeterminados de su esquema, de modo que cada campo del subformulario siempre tiene un valor inicial con sentido.

> **Importante.** En esta versión del panel **el transporte HTTP/2 (`h2`) no está disponible en la lista** — fue excluido del conjunto de redes; para un túnel bidireccional similar a HTTP/2 se usa gRPC, y para el transporte moderno enmascarado en HTTP — XHTTP. El transporte **Hysteria** (`hysteria`) no se selecciona a través de esta lista: está vinculado de forma fija al protocolo Hysteria y aparece automáticamente cuando el propio inbound se crea con el protocolo Hysteria (ver apartado 6.8).

A continuación se describe cada red y cada uno de sus campos por separado.

---

### 6.2. RAW / TCP (`tcpSettings`)

Transporte TCP básico. De forma predeterminada el tráfico se transmite «tal cual»; opcionalmente se puede enmascarar como un intercambio HTTP/1.1 ordinario.

| Campo | Valor predeterminado | Descripción |
| --- | --- | --- |
| Proxy Protocol (`acceptProxyProtocol`) | `false` (desact.) | Aceptar la cabecera PROXY protocol de un balanceador/proxy superior |
| Ofuscación HTTP (`header.type`) | `none` (desact.) | Activa el enmascaramiento del tráfico como HTTP/1.1 |

#### Proxy Protocol

El interruptor **«Proxy Protocol»** (`acceptProxyProtocol`). Cuando está activado, Xray espera en la conexión entrante la cabecera PROXY protocol y extrae de ella la IP real del cliente. Se activa únicamente si delante del panel hay un proxy inverso o balanceador (por ejemplo, HAProxy o nginx con `send-proxy`) que añada dicha cabecera. Desactivado de forma predeterminada.

#### Ofuscación HTTP (camouflage)

El interruptor **«HTTP Obfuscation»**. Controla el campo `header`:

- **Desactivado** → `header.type = "none"` (el campo `header` simplemente no aparece en el cable). TCP limpio.
- **Activado** → `header.type = "http"`. El tráfico se encuadra simulando una petición y respuesta HTTP/1.1. Al activarlo, el panel rellena inmediatamente los subobjetos `request` y `response` con valores predeterminados.

Cuando la ofuscación HTTP está activada, aparecen los campos de configuración de la petición y respuesta simuladas.

**Cabecera de petición (`header.request`):**

| Campo | Clave | Valor predeterminado | Descripción |
| --- | --- | --- | --- |
| Versión de petición | `request.version` | `1.1` | Versión HTTP en la línea de inicio de la petición |
| Método de petición | `request.method` | `GET` | Método HTTP de la petición simulada |
| Ruta de petición | `request.path` | `/` | Ruta(s). Se introduce como lista de valores separados por coma; en el cable es un array de cadenas. Si se deja vacío, se sustituye por `/` |
| Cabeceras de petición | `request.headers` | `{}` (vacío) | Tabla «Nombre/Valor» de cabeceras HTTP. Se almacena como mapa `nombre → [valores]` (a un mismo nombre pueden corresponder varios valores) |

**Cabecera de respuesta (`header.response`):**

| Campo | Clave | Valor predeterminado | Descripción |
| --- | --- | --- | --- |
| Versión de respuesta | `response.version` | `1.1` | Versión HTTP en la línea de inicio de la respuesta |
| Estado de respuesta | `response.status` | `200` | Código de estado HTTP de la respuesta simulada |
| Motivo de respuesta | `response.reason` | `OK` | Descripción textual del estado (reason-phrase) |
| Cabeceras de respuesta | `response.headers` | `{}` (vacío) | Tabla «Nombre/Valor» de cabeceras de respuesta (mapa `nombre → [valores]`) |

Los campos de cabeceras se editan línea a línea — cada línea define el nombre de la cabecera (`Nombre`) y su valor (`Valor`). Estos parámetros solo se usan para enmascarar el aspecto externo del tráfico; no afectan a la criptografía. Los valores predeterminados (`GET / HTTP/1.1`, respuesta `200 OK`) son adecuados para la mayoría de los escenarios — conviene modificarlos solo si se necesita imitar un sitio o servicio concreto.

**Ejemplo de `streamSettings` para RAW con ofuscación HTTP:**

```json
{
  "network": "tcp",
  "tcpSettings": {
    "acceptProxyProtocol": false,
    "header": {
      "type": "http",
      "request": {
        "version": "1.1",
        "method": "GET",
        "path": ["/"],
        "headers": {
          "Host": ["www.example.com"]
        }
      },
      "response": {
        "version": "1.1",
        "status": "200",
        "reason": "OK"
      }
    }
  }
}
```

Nótese que `path` en el cable es un array de cadenas, y cada cabecera también es un array de valores (`Host → ["www.example.com"]`).

---

### 6.3. mKCP (`kcpSettings`)

mKCP es un transporte fiable sobre UDP. Resulta útil en canales con pérdida de paquetes y alta latencia, pero genera mayor tráfico de control. Todos los valores predeterminados coinciden con los recomendados en xray-core.

| Campo | Clave | Predeterminado | Permitido | Descripción |
| --- | --- | --- | --- | --- |
| MTU | `mtu` | `1350` | 576–1460 | Tamaño máximo de paquete (bytes). Se reduce ante problemas de fragmentación |
| TTI (ms) | `tti` | `20` | 10–100 | Intervalo de transmisión (ms). Menor = menor latencia, pero mayor sobrecarga |
| Uplink (MB/s) | `uplinkCapacity` | `5` | ≥ 0 | Ancho de banda estimado de subida (MB/s) |
| Downlink (MB/s) | `downlinkCapacity` | `20` | ≥ 0 | Ancho de banda estimado de bajada (MB/s) |
| Multiplicador CWND | `cwndMultiplier` | `1` | ≥ 1 | Multiplicador de la ventana de congestión (congestion window) |
| Ventana máx. de envío | `maxSendingWindow` | `2097152` | ≥ 0 | Tamaño máximo de la ventana de envío |

Notas sobre los campos:
- **Uplink / Downlink capacity** determinan la agresividad con la que mKCP ocupa el canal. Se ajustan según el ancho de banda real: valores demasiado altos generan tráfico innecesario; demasiado bajos suponen una infrautilización del canal.
- **TTI** afecta directamente al compromiso «latencia ↔ sobrecarga»: valores menores reducen la latencia pero aumentan el volumen de paquetes de control.
- **MTU** limita el tamaño de un paquete mKCP; reducirlo ayuda en canales donde los paquetes UDP grandes se fragmentan o se pierden.

> En esta versión del panel el campo «seed» (contraseña de ofuscación de mKCP) y el desplegable de **tipo de cabecera/ofuscación** (`none`, `srtp`, `utp`, `wechat-video`, `dtls`, `wireguard`) **no están presentes como campos independientes** en el subformulario de mKCP — la ofuscación a nivel de transporte se ha trasladado al mecanismo general «FinalMask» (incluido el modo `mkcp-legacy`), descrito en la sección correspondiente. El parámetro «congestion» como casilla independiente tampoco está expuesto; el control de congestión se configura mediante `cwndMultiplier` y `maxSendingWindow`.

---

### 6.4. WebSocket (`wsSettings`)

Transporte WebSocket sobre HTTP(S). Se propaga bien a través de CDN y proxies inversos, y se camufla como tráfico web ordinario.

| Campo | Clave | Predeterminado | Descripción |
| --- | --- | --- | --- |
| Proxy Protocol | `acceptProxyProtocol` | `false` | Aceptar la cabecera PROXY protocol del proxy superior (ver apartado 6.2) |
| Host | `host` | `""` (vacío) | Valor de la cabecera HTTP `Host`. Se especifica al trabajar tras CDN/domain-fronting |
| Ruta | `path` | `/` | Ruta en la línea de petición del handshake WebSocket |
| Período de heartbeat | `heartbeatPeriod` | `0` | Intervalo de envío de tramas heartbeat (en segundos). `0` desactiva el heartbeat |
| Cabeceras | `headers` | `{}` (vacío) | Cabeceras HTTP adicionales del handshake. Mapa «Nombre → Valor» (solo valores de cadena, sin arrays) |

Notas:
- **Ruta** debe coincidir en el servidor (inbound) y en el cliente. A menudo, el servidor web enmascara el punto de entrada tras esta ruta.
- **Host** tiene sentido especificarlo si el inbound está detrás de un CDN o se usa domain-fronting; de lo contrario puede dejarse vacío.
- **Período de heartbeat** mantiene la conexión «viva» al pasar por proxies/CDN que terminan agresivamente las sesiones inactivas. Por defecto (`0`) el heartbeat está desactivado.
- A diferencia de RAW, la tabla de cabeceras de WebSocket usa el formato «plano» `nombre → valor` (un único valor por cabecera).

**Ejemplo de `streamSettings` para WebSocket detrás de CDN:**

```json
{
  "network": "ws",
  "wsSettings": {
    "acceptProxyProtocol": false,
    "host": "cdn.example.com",
    "path": "/ray",
    "heartbeatPeriod": 0,
    "headers": {
      "User-Agent": "Mozilla/5.0"
    }
  }
}
```

Los valores de `host` y `path` deben coincidir en el cliente; a diferencia de RAW, el valor de la cabecera aquí es una cadena ordinaria, no un array.

---

### 6.5. gRPC (`grpcSettings`)

El transporte con menor número de parámetros. Tuneliza el tráfico dentro de llamadas gRPC (sobre HTTP/2); es muy compatible con CDN que admiten gRPC. No hay ofuscación de cabeceras.

| Campo | Clave | Predeterminado | Descripción |
| --- | --- | --- | --- |
| Nombre de servicio (`Service Name`) | `serviceName` | `""` (vacío) | Nombre del servicio gRPC (en la práctica, la «ruta» del túnel). Debe coincidir en servidor y cliente |
| Authority | `authority` | `""` (vacío) | Valor de la pseudo-cabecera `:authority` (equivalente a `Host` en HTTP/2). Se especifica al trabajar con CDN/dominio |
| Multi Mode | `multiMode` | `false` (desact.) | Activa la multiplexación de varios flujos gRPC paralelos dentro de una misma conexión |

Notas:
- **Service Name** es el identificador principal del canal gRPC; debe ser idéntico en ambos lados. Un valor vacío es válido, pero normalmente se usa una cadena no obvia para el enmascaramiento.
- **Authority** afecta al `:authority` que se envía en los fotogramas HTTP/2; es necesario principalmente al proxyar a través de CDN.
- **Multi Mode** permite que varios flujos lógicos circulen por una misma conexión física; se activa para mejorar el rendimiento cuando tanto el servidor como el cliente lo soportan.

**Ejemplo de `streamSettings` para gRPC:**

```json
{
  "network": "grpc",
  "grpcSettings": {
    "serviceName": "GunService",
    "authority": "grpc.example.com",
    "multiMode": false
  }
}
```

`serviceName` (aquí `GunService`) actúa como «ruta» del túnel y debe coincidir en servidor y cliente.

---

### 6.6. HTTPUpgrade (`httpupgradeSettings`)

Transporte basado en el mecanismo HTTP `Upgrade` (similar a WebSocket, pero sin el protocolo WebSocket en sí). También se propaga bien a través de proxies y CDN. El conjunto de campos repite el de WebSocket, pero **sin** período de heartbeat.

| Campo | Clave | Predeterminado | Descripción |
| --- | --- | --- | --- |
| Proxy Protocol | `acceptProxyProtocol` | `false` | Aceptar la cabecera PROXY protocol del proxy superior |
| Host | `host` | `""` (vacío) | Valor de la cabecera HTTP `Host` |
| Ruta | `path` | `/` | Ruta de la petición HTTP con cabecera `Upgrade` |
| Cabeceras | `headers` | `{}` (vacío) | Cabeceras HTTP adicionales. Mapa «plano» `nombre → valor` (igual que en WebSocket) |

El propósito de los campos **Host**, **Ruta** y **Cabeceras** coincide con el de WebSocket (apartado 6.4). El heartbeat no está previsto para HTTPUpgrade — es una particularidad de WebSocket.

---

### 6.7. XHTTP / SplitHTTP (`xhttpSettings`)

XHTTP (también conocido como SplitHTTP) es un transporte HTTP multiplexado moderno de xray-core. Divide los flujos ascendente y descendente en peticiones HTTP separadas, lo que resulta muy adecuado para CDN y entornos con restricciones en la duración de las conexiones. No todos los campos son visibles en el editor a la vez: algunos aparecen en función del modo seleccionado (`mode`) y de los interruptores activados.

#### Campos básicos (siempre visibles)

| Campo | Clave | Predeterminado | Descripción |
| --- | --- | --- | --- |
| Host | `host` | `""` (vacío) | Valor de la cabecera HTTP `Host` |
| Ruta | `path` | `/` | Ruta base de las peticiones HTTP |
| Modo (`Mode`) | `mode` | `auto` | Modo de transmisión (ver más abajo) |
| Server Max Header Bytes | `serverMaxHeaderBytes` | `0` | Límite del tamaño de cabeceras de petición en el servidor (bytes). `0` — valor predeterminado de xray-core |
| Padding Bytes | `xPaddingBytes` | `100-1000` | Rango de relleno (padding) aleatorio (en bytes, formato `mín-máx`) para dificultar el análisis de tamaños |
| Cabeceras | `headers` | `{}` (vacío) | Cabeceras HTTP adicionales. Mapa «plano» `nombre → valor` |
| Método HTTP Uplink | `uplinkHTTPMethod` | `""` (Default = POST) | Método HTTP de las peticiones ascendentes. Opciones: vacío (predeterminado POST), `POST`, `PUT`, `GET` (el último solo disponible en modo `packet-up`) |
| Padding Obfs Mode | `xPaddingObfsMode` | `false` (desact.) | Activa la ofuscación avanzada de padding y muestra campos adicionales (ver más abajo) |
| No SSE Header | `noSSEHeader` | `false` (desact.) | No enviar la cabecera `Content-Type: text/event-stream` (SSE). Se activa si interfiere con el paso por nodos intermedios |

#### Campo «Modo» (`mode`)

Lista desplegable con los valores:

| Valor | Descripción |
| --- | --- |
| `auto` | Selección automática de modo (predeterminado) |
| `packet-up` | El flujo ascendente se divide en peticiones HTTP individuales (una petición por paquete) |
| `stream-up` | El flujo ascendente se transmite en una única petición de streaming prolongada |
| `stream-one` | Una única petición de streaming bidireccional compartida |

La elección del modo determina qué campos adicionales se hacen visibles.

**Campos visibles solo con `mode = packet-up`:**

| Campo | Clave | Predeterminado | Descripción |
| --- | --- | --- | --- |
| Máx. peticiones POST en búfer | `scMaxBufferedPosts` | `30` | Máximo de peticiones POST ascendentes en búfer simultáneamente |
| Tamaño máx. de subida (bytes) | `scMaxEachPostBytes` | `1000000` | Tamaño máximo de una petición POST ascendente (bytes) |
| Uplink Data Placement | `uplinkDataPlacement` | `""` (Default = body) | Dónde colocar los datos del flujo ascendente: `body`, `header`, `cookie`, `query` |
| Uplink Data Key | `uplinkDataKey` | `""` | Nombre de clave/cabecera para los datos uplink. Aparece solo si `uplinkDataPlacement` está definido y no es `body` |

**Campo visible solo con `mode = stream-up`:**

| Campo | Clave | Predeterminado | Descripción |
| --- | --- | --- | --- |
| Stream-Up Server | `scStreamUpServerSecs` | `20-80` | Rango de tiempo de mantenimiento de la conexión de streaming del servidor (en segundos, formato `mín-máx`) |

#### Campos de ofuscación de padding (visibles con `xPaddingObfsMode = act.`)

| Campo | Clave | Predeterminado | Descripción |
| --- | --- | --- | --- |
| Padding Key | `xPaddingKey` | `""` (placeholder `x_padding`) | Nombre de clave para el padding |
| Padding Header | `xPaddingHeader` | `""` (placeholder `X-Padding`) | Nombre de la cabecera HTTP en la que se transmite el padding |
| Padding Placement | `xPaddingPlacement` | `""` (Default = queryInHeader) | Dónde colocar el padding: `queryInHeader`, `header`, `cookie`, `query` |
| Padding Method | `xPaddingMethod` | `""` (Default = repeat-x) | Método de generación del padding: `repeat-x` o `tokenish` |

#### Ubicación de sesión y secuencia (siempre visibles)

| Campo | Clave | Predeterminado | Descripción |
| --- | --- | --- | --- |
| Session ID Placement | `sessionIDPlacement` | `""` (Default = path) | Dónde transmitir el identificador de sesión: `path`, `header`, `cookie`, `query` |
| Session ID Key | `sessionIDKey` | `""` (placeholder `x_session`) | Nombre de clave de sesión. Aparece solo si `sessionIDPlacement` está definido y no es `path` |
| Session ID Table | `sessionIDTable` | `""` (placeholder `Base62`) | Conjunto de caracteres para la generación de identificadores de sesión. Se puede elegir uno predefinido del desplegable con autocompletado (`ALPHABET`, `Alphabet`, `BASE36`, `Base62`, `HEX`, `alphabet`, `base36`, `hex`, `number`) o introducir una cadena ASCII arbitraria. Vacío — valor predeterminado de xray-core |
| Session ID Length | `sessionIDLength` | `""` (vacío) | Longitud o rango (por ejemplo `8-16`) de los identificadores generados. Se muestra solo cuando `Session ID Table` está definido; el mínimo debe ser mayor que 0 |
| Sequence Placement | `seqPlacement` | `""` (Default = path) | Dónde transmitir el número de secuencia del paquete: `path`, `header`, `cookie`, `query` |
| Sequence Key | `seqKey` | `""` (placeholder `x_seq`) | Nombre de clave de secuencia. Aparece solo si `seqPlacement` está definido y no es `path` |

Los campos de sesión fueron renombrados en xray-core v26.6.22: antes se llamaban **Session Placement** / **Session Key** (`sessionPlacement` / `sessionKey`) — ahora son **Session ID Placement** / **Session ID Key** (`sessionIDPlacement` / `sessionIDKey`); el nombre anterior ya no es reconocido por el núcleo. Los inbounds guardados antes de la actualización se migran a las nuevas claves automáticamente — no es necesario volver a guardarlos.

Recomendaciones:
- Para la mayoría de las instalaciones es suficiente dejar **Modo = `auto`**, definir **Ruta**/**Host** y (al trabajar con CDN) sincronizarlos con el cliente.
- Los campos de ubicación (`*Placement`/`*Key`) y de ofuscación de padding solo son necesarios para un ajuste fino en escenarios específicos anti-DPI/CDN; cuando están vacíos se utilizan los valores predeterminados de xray-core indicados entre paréntesis.
- Los parámetros relativos al lado cliente/outbound (por ejemplo, intervalos de reintento de POST, tamaños de chunk) no se muestran en el formulario de inbound — el listener del servidor los ignora. El multiplexor XMUX, en cambio, sí está disponible en el formulario de inbound (ver más abajo).

- **Los valores predeterminados de servicio no se escriben.** El panel ya no escribe en las configuraciones XHTTP los valores predeterminados de servicio `scMaxEachPostBytes` y `scMinPostsIntervalMs` — se aplican los valores internos de xray-core. Esto elimina la firma DPI constante (el literal `scMinPostsIntervalMs=30`) por la que anteriormente podía bloquearse el tráfico. Para los inbounds ya guardados, los valores que coinciden con los predeterminados de xray-core no se incluyen en los enlaces ni en la suscripción (no es necesario volver a guardar el inbound); los valores definidos manualmente se conservan.

**Ejemplo de `streamSettings` para XHTTP (modo `auto`):**

```json
{
  "network": "xhttp",
  "xhttpSettings": {
    "host": "xhttp.example.com",
    "path": "/yourpath",
    "mode": "auto",
    "xPaddingBytes": "100-1000"
  }
}
```

Para la mayoría de las instalaciones son suficientes estos cuatro campos; los campos de ubicación de sesión/secuencia y de ofuscación de padding se dejan vacíos — en ese caso se usan los valores predeterminados de xray-core.

#### XMUX (multiplexación de conexiones)

El interruptor **XMUX** (`enableXmux`) activa una capa de multiplexación que distribuye las peticiones paralelas entre un pequeño conjunto de conexiones físicas. Al activarlo se despliegan seis campos configurables (almacenados en `xhttpSettings.xmux`):

| Campo | Clave | Predeterminado | Descripción |
| --- | --- | --- | --- |
| Max Concurrency | `maxConcurrency` | `16-32` | Máximo de peticiones simultáneas por conexión (rango `mín-máx`) |
| Max Connections | `maxConnections` | `0` | Máximo de conexiones físicas (`0` — sin límite) |
| Max Reuse Times | `cMaxReuseTimes` | `""` (vacío) | Cuántas veces reutilizar una conexión |
| Max Request Times | `hMaxRequestTimes` | `600-900` | Máximo de peticiones por conexión (rango) |
| Max Reusable Secs | `hMaxReusableSecs` | `1800-3000` | Tiempo durante el cual una conexión puede reutilizarse (segundos, rango) |
| Keep Alive Period | `hKeepAlivePeriod` | `""` (vacío) | Período keep-alive para mantener la conexión activa |

> **Importante.** No se puede definir **Max Connections** y **Max Concurrency** al mismo tiempo — xray-core rechazará dicha configuración. De forma predeterminada, al activar XMUX el panel asigna `Max Concurrency = 16-32`; si se define **Max Connections** (valor mayor que `0`), el panel eliminará el valor predeterminado de `Max Concurrency` para evitar el conflicto.

---

### 6.8. Transporte Hysteria (`hysteriaSettings`)

El transporte **Hysteria** no se selecciona en la lista «Transporte»: se activa automáticamente cuando el inbound se crea con el protocolo Hysteria y está oculto para otros protocolos (al abandonar el protocolo Hysteria, la red vuelve forzosamente a `tcp`). Parámetros:

| Campo | Clave | Predeterminado | Descripción |
| --- | --- | --- | --- |
| Versión | `version` | `2` (fijo, campo bloqueado) | Versión de Hysteria. Solo se admite Hysteria 2 |
| UDP idle timeout (s) | `udpIdleTimeout` | `60` | Tiempo de espera de inactividad de la sesión UDP (segundos). Rango permitido — 2–600; xray-core rechaza valores fuera de este intervalo al arrancar |
| Masquerade | `masquerade` | desact. (ausente) | Activa el enmascaramiento del listener como servidor HTTP/3 durante el sondeo |

Con **Masquerade** activado aparece la selección de tipo (`type`) y los campos dependientes de él:

- **`""` — default (404 page)**: se devuelve una página 404 estándar (no se requieren campos adicionales).
- **`proxy` (reverse proxy)**: proxy inverso hacia un sitio externo.
  - `url` (**Upstream URL**, placeholder `https://www.example.com`) — dirección de destino;
  - `rewriteHost` (**Reescribir Host**, predeterminado `false`) — sustituir la cabecera `Host`;
  - `insecure` (**Omitir verificación TLS**, predeterminado `false`) — no verificar el certificado TLS del upstream.
- **`file` (serve directory)**: servir archivos desde un directorio.
  - `dir` (**Directorio**, placeholder `/var/www/html`).
- **`string` (fixed body)**: respuesta HTTP fija.
  - `statusCode` (**Código de estado**, predeterminado `0`, rango 0–599);
  - `content` (**Body**) — cuerpo de la respuesta;
  - `headers` (**Cabeceras**) — mapa `nombre → valor`.

Masquerade permite que el inbound basado en Hysteria parezca un servidor HTTP/3 ordinario ante sondeos activos, lo que aumenta el sigilo. De forma predeterminada el enmascaramiento está desactivado.

**Ejemplo de `hysteriaSettings` con proxy inverso (`masquerade` → `proxy`):**

```json
{
  "version": 2,
  "udpIdleTimeout": 60,
  "masquerade": {
    "type": "proxy",
    "url": "https://www.example.com",
    "rewriteHost": true,
    "insecure": false
  }
}
```

En este caso, ante un sondeo, el listener devuelve la respuesta de `https://www.example.com`, enmascarándose como un sitio HTTP/3 ordinario.

---

### 6.9. Parámetros complementarios

Además de la selección de red, en la misma pestaña hay disponibles dos bloques generales independientes del transporte concreto (descritos en detalle en las secciones correspondientes):

- **External Proxy** (`externalProxy`) — lista de direcciones/puertos externos que se sustituyen en los enlaces de suscripción en lugar de la dirección del propio panel.
- **Sockopt** (`sockopt`) — opciones de socket de bajo nivel (TCP Fast Open, mark, estrategia de dominio, proxy transparente, etc.).

#### Real client IP (determinación de la IP real detrás de CDN/relay)

Cuando el inbound está detrás de un intermediario (CDN como Cloudflare, túnel L4/relay u otro panel), Xray ve la dirección del intermediario, no la del visitante real. Esa dirección aparece en la lista de clientes en línea y se usa para el límite de IP por cliente, lo que hace que ambos sean inútiles detrás de un proxy. Para recuperar la IP real, en la sección **Sockopt** del formulario de inbound hay un selector de preset **Real client IP** que combina las configuraciones de `acceptProxyProtocol` y `trustedXForwardedFor`:

| Preset | Qué hace | Cuándo aplicar |
| --- | --- | --- |
| **Off / direct** | Borra ambos campos. | El inbound es accesible directamente por los clientes |
| **Cloudflare CDN** | Establece `sockopt.trustedXForwardedFor = ["CF-Connecting-IP"]`. | WebSocket / HTTPUpgrade / XHTTP / gRPC detrás de CDN Cloudflare (nube naranja) |
| **L4 relay / Spectrum (PROXY)** | Activa `acceptProxyProtocol = true`. | Túnel L4/relay delante del inbound o Cloudflare **Spectrum** |

Los presets son mutuamente excluyentes: elegir uno borra el campo del otro, por lo que un `trustedXForwardedFor` obsoleto no sobrescribe la IP recuperada por PROXY protocol. Debajo del preset siguen visibles el interruptor «raw» **Proxy Protocol** y la lista **Trusted X-Forwarded-For** — el preset simplemente los rellena por usted, y pueden modificarse manualmente si es necesario. Si el preset seleccionado no es compatible con el transporte actual (por ejemplo, PROXY protocol en mKCP), el formulario muestra una advertencia. Estos campos pertenecen únicamente al lado del servidor y **nunca se envían a los clientes en las suscripciones**.

> **Use solo uno.** `acceptProxyProtocol` lee la IP real de la cabecera L4 del PROXY protocol, mientras que `trustedXForwardedFor` la lee de la cabecera HTTP de la petición; mezclarlos manualmente solo tiene sentido si su cadena de intermediarios lo requiere.
- **FinalMask** (`finalmask`) — mecanismo general de ofuscación a nivel de transporte (incluida la ofuscación legacy de mKCP), que reemplaza a los campos individuales «seed»/«header type» dentro de los subformularios de red.

---

## 7. Seguridad de la conexión: TLS, XTLS y REALITY

Cada inbound que admite transmisión a través de un flujo de transporte (VMess, VLESS, Trojan, Shadowsocks, Hysteria) tiene una pestaña **«Seguridad»** en el editor. En ella se configura cómo se cifra y enmascara el canal de transporte. Hay tres modos disponibles, seleccionables mediante botones de radio:

| Modo | Etiqueta en UI | Cuándo está disponible |
|-------|--------------|----------------|
| `none` | **Ninguno** | Siempre (excepto Hysteria, donde TLS es obligatorio) |
| `tls` | **TLS** | Para VMess/VLESS/Trojan/Shadowsocks en redes `tcp`, `ws`, `http`, `grpc`, `httpupgrade`, `xhttp`; para Hysteria — siempre |
| `reality` | **Reality** | Solo para VLESS/Trojan en redes `tcp`, `http`, `grpc`, `xhttp` |

El botón **Ninguno** no aparece si el protocolo es Hysteria (para él TLS es obligatorio). El botón **Reality** aparece solo con una combinación válida de protocolo y red (véase la tabla anterior).

Al cambiar de modo, el panel reconstruye completamente el bloque `streamSettings`: se eliminan los `tlsSettings` y `realitySettings` del modo anterior y se sustituyen por los valores predeterminados del modo seleccionado. En particular, al elegir **Reality**, el panel automáticamente: inserta un par aleatorio de `target` + `serverNames` (SNI) de la lista integrada de dominios populares, genera `shortIds` aleatorios, y realiza una solicitud al servidor para obtener un par de claves X25519 reciente (privateKey/publicKey).

### 7.1. Diferencias: TLS vs XTLS vs REALITY

- **TLS** — cifrado clásico del transporte mediante el protocolo TLS 1.2/1.3. El servidor debe contar con un certificado válido (dominio propio + cadena). El tráfico se presenta como HTTPS normal, aunque para un censor activo el handshake TLS hacia su dominio es reconocible; si se bloquea por SNI o si el certificado no es de confianza, la conexión se bloquea o devuelve un error.

- **XTLS (Vision)** — no es un modo separado en la lista «Seguridad», sino un mecanismo de *flow* sobre TLS **o** Reality. Se activa en el lado del cliente del inbound mediante el campo **Flow** = `xtls-rprx-vision` (o `xtls-rprx-vision-udp443`). Vision está disponible para VLESS en la red `tcp` con `security = tls` o `security = reality`, y también para VLESS sobre transporte `xhttp` con cifrado VLESS habilitado (vlessenc / ML-KEM) — en ese caso el campo **Flow** también puede establecerse en `xtls-rprx-vision`, y el valor se incorpora correctamente al enlace `vless://` (`flow=xtls-rprx-vision`). Vision reduce el «doble cifrado» (TLS-in-TLS) al entregar el payload directamente tras el handshake, lo que acelera la transmisión y mejora el enmascaramiento. Por ello, la combinación **VLESS + Reality + Flow `xtls-rprx-vision`** se considera la configuración moderna recomendada.

> **Restauración automática del flow Vision.** Si en un inbound VLESS/XHTTP el cifrado (ML-KEM, campos decryption/encryption) se activa después de haber añadido clientes, el inbound pasa a ser apto para flow. En esta situación, el panel restaura automáticamente `flow = xtls-rprx-vision` en aquellos clientes que lo requieren pero cuyo campo **Flow** estaba vacío. Antes, en este escenario, Vision desaparecía silenciosamente de las configuraciones, enlaces de invitación y suscripciones (especialmente notorio en inbounds de nodo). No se requiere ninguna acción manual: la corrección se aplica automáticamente al guardar el inbound y una sola vez al actualizar el panel. El comportamiento es conservador: el panel no inventa flows ni sobreescribe un valor establecido explícitamente por el cliente.

- **REALITY** — mecanismo de enmascaramiento sin certificado propio. El servidor «toma prestado» el handshake TLS de un sitio externo real (`target`/`serverNames`), por lo que para un observador la conexión es indistinguible de una solicitud a ese sitio, y no se necesita ningún certificado. La autenticación se basa en un par de claves X25519 y un conjunto de `shortIds`. REALITY es resistente a las pruebas activas (`active probing`) y al bloqueo por SNI, ya que el SNI apunta a un dominio externo real. El precio es que la configuración tiene requisitos más estrictos (un `target` correcto con puerto, sincronización de claves con el cliente).

Regla de elección rápida:
- tiene dominio propio y certificado válido, necesita apariencia HTTPS simple → **TLS** (preferiblemente con Vision);
- no tiene dominio/certificado o necesita la mayor invisibilidad frente a DPI → **REALITY** (con Vision para VLESS/TCP).

### 7.2. Modo «Ninguno» (`none`)

El transporte se transmite sin envoltura TLS: los bloques `tlsSettings` y `realitySettings` se excluyen de `streamSettings`. Este modo no tiene campos adicionales. Es adecuado cuando:
- el inbound escucha solo en `127.0.0.1` y sirve como destino fallback (según la regla del panel, el inbound hijo para fallback debe escuchar en `127.0.0.1` con `security=none`);
- el cifrado/enmascaramiento lo proporciona una capa externa (por ejemplo, un proxy inverso Nginx delante del panel);
- se utiliza un protocolo con cifrado propio (Shadowsocks) en una red interna.

Para inbounds accesibles desde el exterior, no se recomienda el modo «Ninguno».

**Ejemplo: bloque `streamSettings` para TLS en la red `tcp`** (VLESS/Trojan/VMess). Así se ve el resultado después de seleccionar el modo **TLS** y rellenar el SNI y las rutas al certificado:

```json
"streamSettings": {
  "network": "tcp",
  "security": "tls",
  "tlsSettings": {
    "serverName": "vpn.example.com",
    "minVersion": "1.2",
    "maxVersion": "1.3",
    "alpn": ["h2", "http/1.1"],
    "settings": { "fingerprint": "chrome" },
    "certificates": [
      {
        "certificateFile": "/root/cert/vpn.example.com.crt",
        "keyFile": "/root/cert/vpn.example.com.key",
        "ocspStapling": 3600,
        "usage": "encipherment"
      }
    ]
  }
}
```

### 7.3. Modo TLS

Campos del bloque `tlsSettings`. Los valores predeterminados provienen del esquema del panel.

#### Parámetros principales

| Campo (etiqueta) | Valor predeterminado | Descripción |
|----------------|----------------------|----------|
| **SNI** (`serverName`) | `""` (vacío) | Server Name Indication — nombre de dominio presentado en el handshake TLS. Debe coincidir con el dominio del certificado. Texto de ayuda en inglés: «Server Name Indication». |
| **Cipher Suites** (`cipherSuites`) | `""` → **Auto** | Lista de conjuntos de cifrado permitidos. Por defecto vacío — la selección queda a criterio de Xray/Go (opción **Auto**). Cambiar solo si es necesario restringir explícitamente los cifrados. |
| **Versión mín/máx** (`minMaxVersion`) | mín = `1.2`, máx = `1.3` | Versiones mínima y máxima de TLS. Valores disponibles: `1.0`, `1.1`, `1.2`, `1.3`. Se recomienda mantener `1.2`–`1.3`; reducir el mínimo a 1.0/1.1 no es deseable (versiones obsoletas e inseguras). |
| **uTLS** (`settings.fingerprint`) | `chrome` (en el formulario — el elemento **None** = `""` está disponible) | Huella TLS imitada del cliente hello (uTLS fingerprint), para que el handshake parezca el de un navegador popular. Véase la lista a continuación. En TLS el primer elemento de la lista es **None** (`""`), que desactiva la imitación. |
| **ALPN** (`alpn`) | `["h2", "http/1.1"]` | Lista de protocolos de capa de aplicación negociados en TLS (selección múltiple). Valores permitidos: `h3`, `h2`, `http/1.1`. Por defecto se ofrecen `h2` y `http/1.1`. |

Valores posibles de **uTLS fingerprint** (idénticos para TLS y REALITY): `chrome`, `firefox`, `safari`, `ios`, `android`, `edge`, `360`, `qq`, `random`, `randomized`, `randomizednoalpn`, `unsafe`. En el formulario TLS también está disponible la opción vacía **None** (la imitación de huella no se aplica).

Valores disponibles de **Cipher Suites** (TLS 1.3 y conjuntos ECDHE): `TLS_AES_128_GCM_SHA256`, `TLS_AES_256_GCM_SHA384`, `TLS_CHACHA20_POLY1305_SHA256`, `TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA`, `TLS_ECDHE_ECDSA_WITH_AES_256_CBC_SHA`, `TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA`, `TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA`, `TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256`, `TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384`, `TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256`, `TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384`, `TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256`, `TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305_SHA256`.

#### Interruptores TLS

| Interruptor | Predeterminado | Descripción |
|---------------|--------------|----------|
| **Rechazar SNI desconocido** (`rejectUnknownSni`) | desact. (`false`) | Si está activado, el servidor cierra la conexión cuando el SNI presentado por el cliente no coincide con el nombre en el certificado. Aumenta la invisibilidad (el servidor no responde a solicitudes «ajenas»), pero requiere que el SNI del cliente coincida exactamente. |
| **Deshabilitar System Root** (`disableSystemRoot`) | desact. (`false`) | Desactiva el uso del almacén de certificados raíz de confianza del sistema. |
| **Reanudación de sesión** (`enableSessionResumption`) | desact. (`false`) | Activa la reanudación de sesión TLS (session resumption / session tickets). |

#### Parámetros adicionales de TLS (vcn, curvas, registro de claves, ECH Sockopt)

Bajo la configuración principal de TLS hay campos adicionales.

| Campo (etiqueta) | Predeterminado | Descripción |
|----------------|--------------|----------|
| **Verify Peer Cert By Name** (`settings.verifyPeerCertByName`) | `""` | Nombres (separados por comas) con los que el cliente verifica el certificado del servidor en lugar del SNI. Es el sustituto moderno del campo `allowInsecure` eliminado de Xray tras el 2026-06-01. Es solo para el panel: no se escribe en el config de xray del servidor, pero se incluye en los enlaces de invitación y suscripciones (`vcn=…`) para que el cliente lo aplique. Texto de ayuda: `example.com`. |
| **Curve Preferences** (`curvePreferences`) | `""` | Restricción y orden de las curvas de intercambio de claves TLS, en orden de preferencia (por ejemplo `X25519MLKEM768`, `X25519`). Vacío — se usan los valores predeterminados de Xray-core. |
| **Master Key Log** (`masterKeyLog`) | `""` | Ruta para registrar las claves maestras TLS en formato `SSLKEYLOGFILE` (para descifrar el tráfico en Wireshark durante la depuración). Texto de ayuda: `/path/to/sslkeylog.txt`. En producción, dejar vacío — el archivo permite descifrar todo el tráfico. |
| **ECH Sockopt** (`echSockopt`) | desact. | Interruptor con parámetros de socket para la conexión a través de la cual Xray solicita la lista de configuraciones ECH. Al activarlo están disponibles: **Dialer Proxy** (`dialerProxy` — enrutar la solicitud a través del outbound especificado por etiqueta), **Domain Strategy** (`domainStrategy`), **TCP Fast Open** (`tcpFastOpen`), **Multipath TCP** (`tcpMptcp`). Dejar desactivado si no es necesario. |

Los campos `verifyPeerCertByName`, `curvePreferences`, `masterKeyLog` y `echSockopt` se encuentran en el nivel superior de `tlsSettings` y sobreviven al «recorte» de campos del panel al guardar la configuración.

#### Certificados

La sección **Certificado SSL** (en UI el encabezado es «Certificado SSL») se define como una lista: el botón **+** añade una nueva entrada de certificado, el botón **− Eliminar** la quita (el botón de eliminación solo está disponible si hay más de una entrada). Por defecto, al activar TLS se crea una entrada vacía.

Para cada entrada, el interruptor de modo de entrada (`useFile`):

- **Ruta al certificado** (valor `useFile = true`, predeterminado) — se especifican las rutas a los archivos en el servidor:
  - **Clave pública** (`certificateFile`) — ruta al archivo del certificado (`.crt`/`.pem`);
  - **Clave privada** (`keyFile`) — ruta al archivo de clave privada (`.key`).
- **Contenido del certificado** (valor `useFile = false`) — el contenido se pega directamente en los campos (áreas de texto multilínea):
  - **Clave pública** (`certificate`) — contenido PEM del certificado;
  - **Clave privada** (`key`) — contenido PEM de la clave.

Bajo los campos del modo «Ruta al certificado» hay dos botones:
- **Establecer certificado del panel** — inserta en los campos las rutas al certificado SSL propio del panel. Para un inbound en el panel central se usa su certificado (`POST /panel/setting/all` → `webCertFile`/`webKeyFile`); para un inbound asignado a un nodo, se usa el certificado del propio nodo (`GET /panel/api/nodes/webCert/{nodeId}`), porque las rutas del panel central no existen en el nodo. Si no hay certificado configurado, se muestra una advertencia: «*El panel no tiene certificado configurado. Primero instálelo en Configuración.*» (el propio certificado del panel se establece en la sección «Configuración → Seguridad»).
- **Limpiar** — borra ambas rutas.

Campos adicionales de cada entrada de certificado:

| Campo | Predeterminado | Descripción |
|------|--------------|----------|
| **OCSP Stapling** (`ocspStapling`) | `0` (desact.) | Intervalo de actualización del OCSP stapling en segundos (mínimo `0`). Para nuevos inbounds está desactivado por defecto (`0`): esto elimina errores en los logs de xray para certificados sin respondedor OCSP (por ejemplo, Let's Encrypt, que abandonó OCSP). Activar solo para certificados que admiten stapling. |
| **Carga única** (`oneTimeLoading`) | desact. (`false`) | Si está activado, el certificado se lee del disco una sola vez al iniciar y no se vuelve a leer si el archivo cambia. |
| **Opción de uso** (`usage`) | `encipherment` | Propósito del certificado. Valores permitidos: `encipherment` (cifrado — certificado de servidor habitual), `verify` (verificación), `issue` (emisión — el servidor firma/emite certificados propios). |
| **Build Chain** (`buildChain`) | desact. (`false`) | Se muestra **solo** cuando `usage = issue`. Construir la cadena de certificados. |

> No existe un botón separado de certificado autofirmado en el editor de inbound: el panel no genera un certificado autofirmado al vuelo para el inbound. El certificado se especifica mediante ruta/contenido o se obtiene de la configuración del panel con el botón «Establecer certificado del panel». La emisión/obtención del certificado SSL del propio panel (incluida la carga de archivos y la vinculación al dominio) se realiza en la sección **Configuración → Seguridad**; no hay endpoints ACME/Let's Encrypt para inbounds aquí.

#### ECH y anclaje de certificado (campos avanzados de TLS)

| Campo | Predeterminado | Descripción |
|------|--------------|----------|
| **ECH key** (`echServerKeys`) | `""` | Claves de servidor para Encrypted Client Hello. |
| **ECH config** (`settings.echConfigList`) | `""` | Lista de configuraciones ECH (parte del cliente, se incluye en el enlace). |
| **SHA-256 del certificado del par** (`settings.pinnedPeerCertSha256`) | `[]` | Hashes SHA-256 del certificado del par (cadenas hex, separadas por comas). Texto de ayuda literal: «*Hashes SHA-256 del certificado del par como cadena hexadecimal (ej. e8e2d3…), separados por comas. Solo para el panel — no se escribe en el config de xray del servidor, pero se incluye en los enlaces de invitación para que los clientes puedan anclar el certificado.*» |

Botones:
Junto al campo **SHA-256 del certificado del par** hay dos botones de autocompletado:
- **Fill from this inbound's certificate** (icono de escudo) — inserta el hash SHA-256 del certificado de este propio inbound (se obtiene localmente a través del endpoint `getCertHash`).
- **Fetch the hash by pinging the SNI (xray tls ping)** (icono de descarga) — obtiene el hash del certificado activo del servidor realizando una conexión TLS al SNI indicado (en el servidor se llama a `getRemoteCertHash`). El campo **SNI** (`serverName`) debe estar relleno — de lo contrario se muestra el aviso «*Set the SNI (serverName) first to ping the remote certificate.*»

Los hashes obtenidos se añaden al campo (separados por comas) y se incluyen en los enlaces de invitación para que el cliente pueda anclar el certificado.
- **Obtener nuevo certificado ECH** — solicita al servidor un nuevo par ECH para el SNI actual (`POST /panel/api/server/getNewEchCert`, en el servidor se ejecuta `xray tls ech --serverName <SNI>`); rellena los campos **ECH key** y **ECH config**.
- **Limpiar** — vacía ambos campos ECH.

### 7.4. Modo REALITY

Campos del bloque `realitySettings`. REALITY no usa certificado SSL: en su lugar utiliza el handshake TLS prestado de un dominio externo y un par de claves X25519.

#### Parámetros de enmascaramiento

| Campo (etiqueta) | Valor predeterminado | Descripción |
|----------------|----------------------|----------|
| **Mostrar** (`show`) | desact. (`false`) | Salida de depuración de REALITY en los logs de Xray. Normalmente se deja desactivado. |
| **Xver** (`xver`) | `0` | Versión del protocolo PROXY transmitida al backend (`0` — desactivado). Mínimo `0`. |
| **uTLS** (`settings.fingerprint`) | `chrome` | Huella TLS imitada (la misma lista que en TLS, pero sin la opción vacía None). |
| **Destino** (`target`) | `""` (el panel inserta uno aleatorio al activar) | **Campo obligatorio.** El dominio real cuyo handshake TLS toma prestado REALITY. Texto de ayuda literal: «*Obligatorio. Debe incluir el puerto (ej. example.com:443). Sin puerto, Xray-core no arranca.*» La validación del panel comprueba la presencia y corrección del puerto; de lo contrario se muestran errores «El destino de REALITY es obligatorio» / «El destino de REALITY debe incluir el puerto…» / «El destino de REALITY tiene un puerto no válido». El botón de actualización junto al campo inserta un par aleatorio de la lista integrada. |
| **SNI** (`serverNames`) | `[]` (se inserta junto al destino) | Lista de SNI permitidos (entrada múltiple con etiquetas). Debe corresponder al dominio en **Destino**. El botón de actualización inserta el SNI junto con el destino aleatorio. |
| **Diferencia máx. de tiempo (ms)** (`maxTimediff`) | `0` | Diferencia máxima de reloj admisible entre cliente y servidor en milisegundos (`0` — sin límite). Mínimo `0`. |
| **Versión mín. del cliente** (`minClientVer`) | `""` | Versión mínima del cliente Xray (texto de ayuda `25.9.11`). Vacío — sin restricción. |
| **Versión máx. del cliente** (`maxClientVer`) | `""` | Versión máxima del cliente Xray. Vacío — sin restricción. |
| **Short IDs** (`shortIds`) | `[]` (se generan al activar) | Lista de identificadores cortos (hex) que distinguen a los clientes. Entrada múltiple con etiquetas; el botón de actualización genera un conjunto aleatorio. |
| **SpiderX** (`settings.spiderX`) | `/` | Ruta del «spider» (parte del cliente de REALITY), usada al imitar la solicitud al sitio externo. Se incluye en el enlace de invitación. |

**Destino** (`target`) y **SNI** (`serverNames`) al activar REALITY y al pulsar el botón de actualización se rellenan con un par aleatorio de la lista integrada del panel: `www.amazon.com`, `aws.amazon.com`, `www.oracle.com`, `www.nvidia.com`, `www.amd.com`, `www.intel.com`, `www.sony.com` (cada uno con el puerto `:443`). Elija un sitio HTTPS externo estable y de gran tráfico que no esté detrás de su propio servidor.

**Ejemplo: bloque `streamSettings` para REALITY en la red `tcp`** (VLESS). No se necesita certificado — en su lugar se usa el dominio prestado y el par de claves X25519:

```json
"streamSettings": {
  "network": "tcp",
  "security": "reality",
  "realitySettings": {
    "show": false,
    "xver": 0,
    "dest": "www.nvidia.com:443",
    "serverNames": ["www.nvidia.com"],
    "privateKey": "YOUR_X25519_PRIVATE_KEY",
    "shortIds": ["", "6ba85179e30d4fc2"],
    "settings": {
      "publicKey": "YOUR_X25519_PUBLIC_KEY",
      "fingerprint": "chrome",
      "spiderX": "/"
    }
  }
}
```

Aquí el campo **Destino** (`target`) del panel corresponde a `dest` en el config de Xray generado. Si un inbound REALITY fue creado con el destination en la clave `dest` (por versiones antiguas del panel, a través de la API o herramientas externas), el panel al cargarlo normaliza `dest` → `target` cuando `target` está vacío — por lo que dicho inbound se carga correctamente, el campo **Destino** no queda vacío y al volver a guardar no se rompe el REALITY funcional.

#### Claves REALITY (X25519)

| Campo | Predeterminado | Descripción |
|------|--------------|----------|
| **Clave pública** (`settings.publicKey`) | `""` | Clave pública X25519 (el cliente la incluye en su configuración/enlace). |
| **Clave privada** (`privateKey`) | `""` | Clave privada X25519 (se almacena solo en el servidor). |

Botones bajo las claves:
- **Obtener nuevo certificado** — solicita al servidor un nuevo par de claves (`GET /panel/api/server/getNewX25519Cert`; en el servidor se ejecuta `xray x25519`), rellena **Clave privada** y **Clave pública**. Este mismo par se genera automáticamente al activar el modo REALITY por primera vez.

**Ejemplo: obtener un par de claves X25519 mediante la API** (fuera del formulario, por ejemplo para un script). La solicitud devuelve la clave privada y la pública:

```bash
curl -s -b cookie.txt https://your-panel:2053/panel/api/server/getNewX25519Cert
# Ответ:
# {"success":true,"obj":{"privateKey":"...","publicKey":"..."}}
```

`cookie.txt` — archivo de cookie de sesión obtenido tras iniciar sesión mediante `POST /login`.
- **Limpiar** — vacía ambas claves.

#### Firma post-cuántica ML-DSA-65 (mldsa65)

Capa adicional (opcional) de autenticación post-cuántica de REALITY:

| Campo | Predeterminado | Descripción |
|------|--------------|----------|
| **mldsa65 Seed** (`mldsa65Seed`) | `""` | Seed de clave ML-DSA-65 del servidor. |
| **mldsa65 Verify** (`settings.mldsa65Verify`) | `""` | Valor de verificación (parte del cliente, se incluye en el enlace). |

Botones:
- **Obtener nuevo Seed** — solicita un nuevo par (`GET /panel/api/server/getNewmldsa65`; en el servidor se ejecuta `xray mldsa65`), rellena **mldsa65 Seed** y **mldsa65 Verify**.
- **Limpiar** — vacía ambos campos.

#### Límite de velocidad del fallback y registro de claves REALITY

En la configuración de REALITY hay disponible un límite de velocidad del tráfico fallback — evita que las pruebas activas usen el servidor como canal gratuito hacia el dominio prestado. El ajuste se configura por separado para dos direcciones — **Limit Fallback Upload** y **Limit Fallback Download** (`limitFallbackUpload` / `limitFallbackDownload`), cada una con el mismo conjunto de campos:

| Campo (etiqueta) | Predeterminado | Descripción |
|----------------|--------------|----------|
| **After Bytes** (`afterBytes`) | `0` | Cuántos bytes dejar pasar a velocidad plena antes de comenzar a limitar. `0` — limitar desde el primer byte. |
| **Bytes Per Sec** (`bytesPerSec`) | `0` | Techo de velocidad del tráfico fallback en bytes por segundo tras el umbral. `0` — sin límite (desactiva esta dirección). |
| **Burst Bytes Per Sec** (`burstBytesPerSec`) | `0` | Reserva para ráfagas breves por encima de la velocidad constante (tamaño del token-bucket). Si es menor que **Bytes Per Sec**, se eleva a ese valor. |

Allí mismo se ha añadido el campo **Master Key Log** (`masterKeyLog`) — ruta para registrar las claves maestras TLS en formato `SSLKEYLOGFILE` para la depuración en Wireshark; en producción, dejar vacío.

### 7.5. Recomendaciones prácticas de configuración

1. **VLESS + Reality (recomendado):** cree un inbound VLESS en la red `tcp`, en la pestaña «Seguridad» seleccione **Reality** — el panel insertará automáticamente `target`/SNI aleatorios, `shortIds` y generará las claves X25519. Si es necesario, pulse «Obtener nuevo certificado» para obtener su propio par de claves. Para los clientes VLESS, active **Flow** = `xtls-rprx-vision` (XTLS Vision) — esto proporcionará el máximo rendimiento e invisibilidad.

**Ejemplo: enlace de cliente final VLESS + Reality + Vision.** Así se ve el enlace de invitación que genera el panel para dicho inbound (los valores de claves/ID son ilustrativos):

```text
vless://uuid-клиента@1.2.3.4:443?type=tcp&security=reality&pbk=ПУБЛИЧНЫЙ_КЛЮЧ&fp=chrome&sni=www.nvidia.com&sid=6ba85179e30d4fc2&spx=%2F&flow=xtls-rprx-vision#my-reality
```

Aquí `pbk` es la clave pública X25519, `sni` es el dominio prestado de **Destino**, `sid` es uno de los **Short IDs**, `flow=xtls-rprx-vision` es XTLS Vision activado.
2. **TLS con dominio propio:** seleccione **TLS**, rellene **SNI** con el nombre del dominio, añada el certificado (mediante ruta a los archivos o contenido), o pulse «Establecer certificado del panel» si el dominio y el certificado ya están configurados en «Configuración → Seguridad». Deje **Versión mín/máx** = `1.2`–`1.3` y **uTLS** = `chrome` para imitar un navegador normal.
3. No deje el modo **Ninguno** para inbounds expuestos al exterior — úselo solo para destinos fallback locales (`127.0.0.1`) o cuando TLS lo proporcione un proxy externo.
4. Consejo de la interfaz: para la mayoría de los campos avanzados hay un aviso «*Se recomienda dejar la configuración predeterminada*» — modifíquelos solo si comprende las consecuencias.

---

## 8. Clientes

Un cliente es una cuenta de usuario de VPN: un conjunto de credenciales (UUID o contraseña) vinculado a uno o varios inbound, con su propia cuota de tráfico, fecha de expiración y límite de conexiones simultáneas. En este fork, el cliente es una entidad independiente (tabla `clients`): el mismo cliente puede estar vinculado a varios inbound a la vez, conservando el mismo UUID/contraseña y el mismo contador de tráfico. La sección **Clientes** muestra todas las cuentas del panel independientemente del inbound, con búsqueda, filtros, ordenación y operaciones masivas.

### 8.1. Campos del cliente

A continuación se detalla cada campo del editor **Agregar cliente** / **Editar cliente**.

El formulario del cliente está dividido en dos pestañas: **General** (email, vinculación al inbound, límites, plazo, grupo, comentario, etiqueta inversa) y **Credenciales** (UUID/contraseña/auth, Flow, VMess Security). En las etiquetas de los campos, la cuota aparece como **Límite de tráfico (GB)**, y los plazos como **Duración (días)** y **Renovación automática (días)**; los campos **Límite de tráfico (GB)** y **Límite de IP** tienen sugerencias que explican que `0` significa «sin restricciones». Al editar un cliente ya existente, el botón de generación de email aleatorio se oculta, y el botón del registro de IP se muestra directamente junto al campo **Límite de IP** e indica el número de direcciones registradas.

| Campo | Clave JSON | Por defecto | Descripción |
|-------|-----------|-------------|-------------|
| Email | `email` | — (obligatorio) | Identificador único del cliente |
| UUID | `id` | generado | Identificador para VMess/VLESS |
| Contraseña | `password` | generada | Contraseña para Trojan/Shadowsocks |
| Autorización | `auth` | generada | Contraseña para Hysteria |
| Flow | `flow` | vacío | Control de flujo (XTLS), solo VLESS |
| VMess Security | `security` | `auto` | Método de cifrado VMess |
| Límite de IP | `limitIp` | `0` (sin límite) | Máximo de IP simultáneas |
| Total enviado/recibido (GB) | `totalGB` | `0` (sin límite) | Cuota de tráfico |
| Fecha de expiración | `expiryTime` | `0` (sin límite) | Fecha de vencimiento |
| Renovación automática | `reset` | `0` (desactivado) | Período de reinicio de tráfico, días |
| ID de usuario de Telegram | `tgId` | `0` (ninguno) | ID numérico de Telegram |
| ID de suscripción | `subId` | generado | Identificador de suscripción |
| Grupo | `group` | vacío | Etiqueta lógica de agrupación |
| Comentario | `comment` | vacío | Nota libre |
| Habilitado | `enable` | `true` | Si la cuenta está activa |

#### Email (identificador)

El campo **Email** es el identificador principal y obligatorio del cliente. A pesar del nombre, no es necesariamente una dirección de correo electrónico: cualquier etiqueta de texto es válida (nombre de usuario, número). El valor debe ser **único** dentro del panel; el intento de crear un segundo cliente con un email ya ocupado se rechaza (`email already in use`), salvo cuando el `subId` también coincide (esto se interpreta como la vinculación del mismo cliente).

El email **no puede dejarse vacío** (`client email is required`) y **no puede contener espacios, los caracteres `/`, `\` ni caracteres de control** («El email no puede contener espacios, '/', '\' ni caracteres de control»). El email participa en la contabilidad de tráfico, en el registro de IP, en la lista de usuarios en línea y en los nombres de las operaciones; no se recomienda cambiarlo a posteriori.

#### UUID / Contraseña / Autorización (credenciales)

El campo de credenciales concreto depende del protocolo del inbound al que se vincula el cliente. Los valores se rellenan automáticamente si se deja el campo vacío:

- **UUID** (campo `id`) — para los protocolos **VMess** y **VLESS**. Si no se especifica, se genera un UUID v4 aleatorio.
- **Contraseña** (campo `password`) — para **Trojan** y **Shadowsocks**. Para Trojan se genera por defecto un UUID sin guiones. Para Shadowsocks se genera una clave de la longitud adecuada en Base64 según el método de cifrado del inbound: 16 bytes para `2022-blake3-aes-128-gcm`, 32 bytes para `2022-blake3-aes-256-gcm` y `2022-blake3-chacha20-poly1305`; para otros métodos, un UUID sin guiones. Si la clave introducida manualmente no es compatible con el método 2022-blake3, será reemplazada por una generada automáticamente.
- **Autorización** (campo `auth`) — contraseña para **Hysteria**. Por defecto, UUID sin guiones.

Dado que un mismo cliente puede estar vinculado a inbound de distintos protocolos, el registro del cliente puede contener simultáneamente UUID, contraseña y auth; para cada protocolo se utiliza su propio campo.

**Ejemplo: cómo aparecen las credenciales del cliente en `settings` de diferentes inbound.** El mismo cliente en un inbound VLESS se identifica por `id`, en Trojan por `password`, en Shadowsocks por `password` (clave Base64):

```json
// фрагмент settings.clients у VLESS-inbound
{ "id": "b831381d-6324-4d53-ad4f-8cda48b30811", "email": "user-a", "flow": "xtls-rprx-vision" }

// тот же клиент в Trojan-inbound
{ "password": "b831381d63244d53ad4f8cda48b30811", "email": "user-a" }

// тот же клиент в Shadowsocks-inbound (метод 2022-blake3-aes-256-gcm)
{ "password": "GPyOaA3f7CO0az53eaQ8eqMfRDjmBlOh7v1u3+Z+pHk=", "email": "user-a" }
```

#### Flow

**Flow** (campo `flow`) — control de flujo XTLS. Aplicable **únicamente a VLESS** y solo cuando el inbound está configurado para XTLS Vision: VLESS sobre transporte **TCP** con security **`tls`** o **`reality`**. El valor permitido es `xtls-rprx-vision` (así como el histórico `xtls-rprx-vision-udp443`); el valor vacío indica ausencia de flow.

Si el inbound no admite XTLS-flow, el flow especificado **se borra silenciosamente** al guardar el cliente: para el mismo cliente vinculado a varios inbound, el flow se aplica solo donde es admisible. Solo conviene modificarlo si se está usando VLESS-Vision de forma deliberada.

#### VMess Security

**VMess Security** (campo `security`) — método de cifrado del payload para VMess. El valor por defecto es `auto` (Xray elige el cifrado automáticamente). Los valores permitidos son los estándar de VMess: `auto`, `aes-128-gcm`, `chacha20-poly1305`, `none`, `zero`. Para otros protocolos este campo no se utiliza.

#### Límite de IP (conexiones simultáneas)

**Límite de IP** (campo `limitIp`) — número máximo de **direcciones IP distintas** desde las que el cliente puede estar conectado simultáneamente. El valor por defecto es `0`, lo que significa **sin restricción**. Con un valor positivo, el panel rastrea las IP activas del cliente y, al superar el límite, desactiva la cuenta mediante una tarea en segundo plano. (A partir de **3.3.1**, el conteo de IP se realiza a través de la API de estadísticas en línea del núcleo Xray y **no requiere** el registro de acceso; en versiones anteriores del núcleo, el panel recurre a la lectura del registro de acceso, que debe estar habilitado.) Utilícelo para impedir compartir una suscripción en muchos dispositivos: por ejemplo, `2` permite dos dispositivos.

El límite de IP se aplica mediante **Fail2ban**, por lo que el campo **Límite de IP** solo está activo si Fail2ban está instalado y funcionando (el panel comprueba su estado mediante `GET /panel/api/server/fail2banStatus`). Si Fail2ban no está instalado, el campo del editor del cliente (y del formulario de adición masiva) queda bloqueado y, al pasar el cursor, aparece una sugerencia con la propuesta de instalar Fail2ban desde el menú bash `x-ui` («Fail2ban is not installed, so the IP limit cannot be enforced. Install Fail2ban from the x-ui bash menu to enable this option.»); en Windows la sugerencia indica que Fail2ban no está disponible allí («Fail2ban is not available on Windows, so the IP limit cannot be enforced.»), y si la función está deshabilitada en el servidor: «The IP limit feature is disabled on this server.». Al actualizar el panel, el límite de IP guardado de los clientes en servidores sin Fail2ban se pone a cero mediante una migración puntual, ya que de todas formas no se aplicaba.

**Ejemplo de valores.** `limitIp: 0` — sin restricción; `limitIp: 1` — estrictamente un dispositivo simultáneo; `limitIp: 3` — hasta tres IP distintas. Al conectarse un cuarto IP activo, la tarea en segundo plano deshabilitará al cliente (`enable = false`) hasta que se ejecute **Restablecer límite de IP**.

Operaciones relacionadas: **Registro de IP** muestra la lista de IP registradas del cliente; cada entrada contiene, además de la propia IP, la hora del último acceso y la etiqueta del nodo (`@ nombre_nodo`) a través del cual se registró la conexión; en configuraciones multipanel se puede ver por qué nodo se conectó el cliente. **Restablecer límite de IP** limpia el registro de IP acumulado para que el cliente pueda volver a conectarse sin esperar a que expiren los registros.

#### Total enviado/recibido (GB) — cuota de tráfico

**Total enviado/recibido (GB)** (campo `totalGB`) — cuota total de tráfico (envío + recepción). El valor por defecto `0` significa **sin límite**. Al alcanzar la cuota (`up + down >= total`), el cliente se considera **agotado** (depleted) y se desactiva. En la interfaz se introduce habitualmente en gigabytes; en la base de datos se almacena en bytes.

En la lista de clientes, la columna **Tráfico** muestra una barra de uso con color: el volumen de tráfico consumido, la etiqueta del límite (o el símbolo ∞ en caso de ilimitado) y una sugerencia al pasar el cursor con el desglose de enviado/recibido y el saldo restante. El mismo indicador compacto se muestra en las tarjetas de clientes en dispositivos móviles.

#### Fecha de expiración (Expiry)

**Fecha de expiración** (campo `expiryTime`) define el momento de vencimiento de la cuenta. El campo tiene tres modos:

- **Sin límite** — `0`. El cliente nunca expira por tiempo.
- **Fecha concreta** — timestamp Unix positivo (en milisegundos). Al llegar la fecha (`expiryTime <= ahora`), el cliente se considera expirado (expired) y se desactiva. En la interfaz se especifica habitualmente seleccionando una fecha o indicando una duración en días (**Duración**, unidad — **Días**).
- **Inicio tras el primer uso** — valor **negativo** que codifica la duración. Mientras el cliente no haya transmitido ningún byte, el plazo permanece negativo («inicio diferido»). En el primer ciclo de contabilidad de tráfico, el panel lo convierte en una fecha absoluta: `ahora + |duración|`. Esto permite vender, por ejemplo, «30 días desde el primer acceso», sin conocer de antemano cuándo se activará el cliente. La conversión se realiza una sola vez por email, para que todos los inbound vinculados reciban el mismo plazo.

**Ejemplo de codificación del plazo.** Fecha fija 1 de marzo de 2026, 00:00 UTC → `expiryTime: 1772323200000` (timestamp positivo en milisegundos). «30 días desde el primer acceso» → `expiryTime: -2592000000` (valor negativo, `30 × 24 × 60 × 60 × 1000`); al producirse el primer byte de tráfico, el panel lo reemplazará por `ahora + 2592000000`. Sin límite → `expiryTime: 0`.

#### Renovación automática (período de reinicio de tráfico del cliente)

El campo **Renovación automática** (campo `reset`) es el período de renovación/reinicio automático en días. Sugerencia: «Renovación automática tras la expiración. (0 = desactivado) (unidad: día)».

- `0` — renovación automática **desactivada** (valor por defecto). Al expirar, el cliente simplemente queda agotado.
- `> 0` — la tarea en segundo plano, al expirar el plazo, **reinicia los contadores de tráfico a cero** (`up = down = 0`), **adelanta la fecha de expiración** en `reset` días (si es necesario, varios períodos, hasta que el nuevo plazo quede en el futuro) y, si es necesario, vuelve a **habilitar** al cliente. Esto implementa una suscripción periódica (por ejemplo, mensual). La renovación automática **no se aplica a los inbound en nodos** (`node_id IS NOT NULL`).

Consecuencia importante: los clientes con `reset > 0` se **excluyen** del concepto de «agotado» en las operaciones de eliminación masiva; su tráfico/plazo se restablecen por la renovación automática y no convierten la cuenta en candidata a eliminación.

#### ID de usuario de Telegram

**ID de usuario de Telegram** (campo `tgId`) — identificador numérico de Telegram del usuario para vincularlo al bot de Telegram integrado del panel (notificaciones, consulta autónoma de estadísticas). Sugerencia: «ID numérico de usuario de Telegram (0 = ninguno)». El valor por defecto `0` indica que no hay vinculación. Por este campo se puede filtrar (**Con** / **Sin**).

#### ID de suscripción (subId)

**ID de suscripción** (campo `subId`) — identificador bajo el cual el cliente se incluye en la **suscripción** (subscription). Todos los clientes con el mismo `subId` se sirven por un único enlace de suscripción. Si el campo se deja vacío al crear el cliente, el panel **genera automáticamente un** `subId` aleatorio (UUID). El valor debe ser **único** entre los clientes con email diferente (`subId already in use`) y está sujeto a las mismas restricciones de caracteres que el email («El ID de suscripción no puede contener espacios, '/', '\' ni caracteres de control»).

Sin `subId`, el enlace de suscripción del cliente no está disponible («Este cliente no tiene subId; el enlace de acceso compartido no está disponible.»).

#### Pestaña Links (enlaces externos y suscripciones)

Además de las pestañas **General** y **Credenciales**, el editor del cliente tiene una tercera pestaña **Links** (sugerencia: «Add third-party share links and remote subscription URLs to include in this client's subscription.»). En ella, con el botón **Add External Link** se añaden enlaces de terceros (`vless://`, `vmess://`, `trojan://`, `ss://`, `hysteria2://`, `wireguard://`), y con el botón **Add External Subscription** se añaden URLs de suscripciones remotas (por ejemplo, `https://provider.example/sub/…`).

Todo lo anterior se mezcla en la respuesta de suscripción de ese cliente (formatos raw, JSON y Clash): los enlaces se añaden tal cual, y las suscripciones remotas las descarga el panel periódicamente (con caché y tiempo de espera breve) combinando sus configuraciones con las propias. Así, en un único enlace de suscripción del cliente se pueden servir juntos los servidores propios y configuraciones externas adicionales.

#### Grupo

**Grupo** (campo `group`) — etiqueta lógica para agrupar clientes relacionados. Sugerencia: «Etiqueta lógica para agrupar clientes relacionados (p.ej., equipo, cliente, región). Se puede filtrar desde la barra de herramientas.», marcador de posición — «p.ej., customer-a». El campo es opcional (vacío por defecto). Se puede filtrar por grupo la lista y realizar operaciones masivas; para quitar la etiqueta de un cliente se usa la acción **Desagrupar**.

También se puede quitar el grupo directamente desde el editor de un cliente: si se borra el campo **Grupo** y se guarda, la etiqueta se elimina correctamente y el cliente deja de aparecer bajo el grupo anterior.

#### Comentario

**Comentario** (campo `comment`) — nota de texto libre para el administrador (vacío por defecto). El contenido se incluye en la búsqueda y está disponible para filtrar (**Con** / **Sin** comentario).

#### Habilitado

**Habilitado** (campo `enable`) — indicador de actividad de la cuenta. Por defecto **habilitado** (`true`); al crear, aunque no se transmita el indicador, el panel fuerza `true`. Un cliente deshabilitado (`enable = false`) no puede conectarse y en el resumen se clasifica como **inactivo** (deactive). El panel deshabilita automáticamente a los clientes que han agotado la cuota, han expirado o han superado el límite de IP.

#### Campos de solo lectura

En la tarjeta del cliente también se muestran campos de servicio: **Creado** (`created_at`) y **Actualizado** (`updated_at`) — marcas de tiempo de creación y última modificación, se rellenan automáticamente y no son editables. El campo **Etiqueta inversa** (`reverse`) — etiqueta Reverse opcional para el proxy inverso simple VLESS («Etiqueta Reverse opcional»).

### 8.2. Vinculación al inbound

Cada cliente debe estar vinculado a al menos un inbound; al crear se requiere un mínimo de uno (`at least one inbound is required`). En el editor este campo se llama **Entrantes vinculados** con la sugerencia **Seleccione uno o más entrantes**.

- **Vincular** — añadir el cliente a los inbound seleccionados (mismo UUID/contraseña y tráfico compartido). Las vinculaciones existentes se conservan.
- **Desvincular** — quitar el cliente de los inbound seleccionados. El registro del cliente se conserva (para eliminarlo por completo use **Eliminar**). Los pares en los que el cliente no estaba vinculado se omiten silenciosamente.

Al guardar un cliente vinculado a varios inbound, los campos incompatibles con el protocolo/transporte concreto (por ejemplo, Flow fuera de VLESS-Vision) se ajustan automáticamente a los valores admisibles para cada inbound.

Encima de la lista de selección de inbound (en el formulario del cliente, al añadir clientes masivamente y en las ventanas de vinculación/desvinculación masiva) hay botones **Seleccionar todos** y **Limpiar**. En estas listas cada inbound lleva su nota (remark) si está definida, o en caso contrario la etiqueta del inbound.

### 8.3. Operaciones sobre el cliente

Para un cliente individual (a través de la tarjeta **Información del cliente** o el menú contextual **Acciones**) están disponibles:

#### Ver información, código QR y enlace

- **Información del cliente** — tarjeta con todos los campos, tráfico usado/restante (**Saldo**), fecha de expiración e inbound vinculados.

La consulta de un cliente a través de la API (`GET /panel/api/clients/get/:email`) devuelve, junto a los campos `client` e `inboundIds`, adicionalmente `usedTraffic` — el tráfico realmente consumido (enviado + recibido, incluyendo los datos de los nodos), lo que facilita comparar el consumo con la cuota `totalGB`.
- **Código QR** y **Enlace** — enlace de configuración del cliente para importarlo en la aplicación cliente. Se genera a partir de todos los inbound vinculados con protocolo compatible (`GET /links/:email`). Si no hay enlaces apropiados: «No hay enlaces para compartir — vincule primero el cliente a un entrante con protocolo compatible.».
- **Enlace de suscripción** — URL de suscripción por `subId` (`GET /subLinks/:subId`). Disponible solo si el cliente tiene `subId` y el servicio de suscripción está habilitado en **Configuración del panel → Suscripción** (de lo contrario «El servicio de suscripción está desactivado.»). Adicionalmente se proporciona la **URL de suscripción JSON**.

#### Restablecer tráfico

**Restablecer tráfico** (`POST /resetTraffic/:email`) pone a cero los contadores de envío/recepción (`up`, `down`) de un cliente concreto. La cuota (`totalGB`) y la fecha de expiración **no se ven afectadas** — solo se borra el volumen consumido. Notificación: «Tráfico restablecido». Si el cliente no está vinculado a ningún inbound: «Vincule primero este cliente a un entrante.».

El botón **Restablecer tráfico** también está disponible desde el formulario de edición del cliente — en el panel inferior, junto a **Cancelar** / **Guardar** (se solicita confirmación antes de restablecer). Si el cliente fue deshabilitado por agotamiento de tráfico, el restablecimiento (tanto individual como masivo) lo **habilita automáticamente** de nuevo (`enable = true`) y propaga inmediatamente el cambio a los nodos; no es necesario volver a habilitarlo manualmente en el maestro y los nodos.

#### Restablecer límite de IP

Limpia el registro de IP acumulado del cliente (`POST /clearIps/:email`) para levantar el bloqueo temporal por superación del límite de conexiones simultáneas. Notificación: «El registro fue limpiado».

#### Eliminar

**Eliminar** (`POST /del/:email`) — eliminación completa del cliente. Diálogo de confirmación: título «¿Eliminar al cliente {email}?», texto «El cliente será eliminado de todos los entrantes vinculados y su registro de tráfico será destruido. Esta acción no se puede deshacer.». La eliminación desvincula al cliente de **todos** los inbound y destruye su registro de tráfico. Notificación: «Cliente eliminado».

### 8.4. Operaciones masivas

En la lista de clientes se pueden marcar varios registros (**Seleccionar todo**, **Limpiar todo**); contador — «{count} seleccionados». Para los seleccionados están disponibles:

- **Eliminar ({count})** (`POST /bulkDel`) — eliminación grupal. Confirmación: «¿Eliminar {count} clientes?», «Cada cliente seleccionado se elimina de todos los entrantes vinculados y su registro de tráfico se destruye. Esta acción no se puede deshacer.». Notificación: «Clientes eliminados: {count}»; en caso de fallo parcial — «Eliminados: {ok}, fallidos: {failed}».
- **Editar ({count})** / **Ajuste** (`POST /bulkAdjust`) — modificación masiva del plazo y/o la cuota. Diálogo «Editar {count} clientes» con la sugerencia «Los valores positivos suman, los negativos restan. Los clientes con plazo o tráfico ilimitado se omiten para el campo correspondiente.». Campos: **Agregar días**, **Agregar tráfico (GB)** y **Set flow**. Lógica:
  - **Plazo:** los clientes con plazo ilimitado (`expiryTime == 0`) se omiten («unlimited expiry»); para los clientes con fecha, el plazo se desplaza el número de días indicado; para los clientes en modo «tras el primer uso» (plazo negativo), se ajusta la duración de espera. La reducción que supera el saldo disponible se omite («reduction exceeds remaining time/delay window»).
  - **Tráfico:** los clientes con ilimitado (`totalGB == 0`) se omiten («unlimited traffic»); en caso contrario, la cuota cambia en el volumen indicado, sin bajar de cero.
  - **Flow:** la lista desplegable **Set flow** permite establecer o borrar el XTLS flow en todos los clientes seleccionados a la vez. Por defecto se selecciona **No change** (sin cambios). La opción **Disable (clear flow)** borra el flow, y los valores `xtls-rprx-vision` y `xtls-rprx-vision-udp443` establecen el vision-flow correspondiente. La asignación del vision-flow solo se aplica a los inbound que admiten flow; los inbound incompatibles no se modifican y se marcan como omitidos, mientras que el borrado del flow siempre está permitido.
  - Si no se especifican días, tráfico ni flow: «Indique días, tráfico o flow antes de aplicar.». Notificación: «Modificados: {count}» / «Modificados: {ok}, omitidos: {skipped}».

**Ejemplo: ampliar los clientes seleccionados 30 días y añadir 50 GB.** En el diálogo **Editar** indique **Agregar días** = `30`, **Agregar tráfico (GB)** = `50`. Para, al contrario, restar una semana y reducir la cuota en 10 GB, introduzca valores negativos: **Agregar días** = `-7`, **Agregar tráfico (GB)** = `-10` (los clientes con plazo ilimitado o sin límite de tráfico en el campo correspondiente serán omitidos).
- **Vincular ({count})** / **Desvincular ({count})** (`POST /bulkAttach` / `bulkDetach`) — vinculación/desvinculación masiva de los clientes seleccionados a los inbound seleccionados. Los destinos son solo inbound multiusuario. Resultado de la desvinculación: «Desvinculados {detached}, omitidos {skipped}.».
- **Suscripciones ({count})** — tabla resumen de URLs de suscripción y suscripciones JSON de los clientes seleccionados con el botón **Copiar todas**. Si ninguno tiene subId: «Ninguno de los clientes seleccionados tiene ID de suscripción.».
- **Agregar al grupo** y **Desagrupar** — asignación y eliminación de la etiqueta de grupo.

- **Habilitar ({count})** / **Deshabilitar ({count})** (`POST /bulkEnable` / `bulkDisable`) — habilitación y deshabilitación masiva de los clientes seleccionados. **Habilitar** activa cada cliente seleccionado en todos los inbound vinculados; los clientes con cuota de tráfico agotada o plazo expirado serán deshabilitados de nuevo automáticamente. **Deshabilitar** priva inmediatamente a los clientes del acceso, pero sus registros y el tráfico acumulado se conservan. Antes de ejecutar, el panel solicita confirmación y, tras la operación, muestra una notificación con el número de clientes procesados y, si los hay, con el número de aquellos para los que la acción falló.

#### Reinicio de tráfico y eliminación por estado

- **Restablecer tráfico de todos los clientes** (`POST /resetAllTraffics`) — pone a cero los contadores `up`/`down` de **todos** los clientes del panel. Confirmación: «¿Restablecer el tráfico de todos los clientes?» y «Los contadores de envío/recepción de todos los clientes se restablecen a cero. Las cuotas y las fechas de expiración no se ven afectadas. Esta acción no se puede deshacer.». Notificación: «Tráfico de todos los clientes restablecido».
- **Eliminar agotados** (`POST /delDepleted`) — elimina a todos los clientes que tienen **cuota agotada** (`total > 0 and up + down >= total`) **o plazo expirado** (`expiry_time > 0 and expiry_time <= ahora`), con la condición de `reset = 0` (los clientes con renovación automática no se tocan). Confirmación: «¿Eliminar los clientes agotados?», «Se eliminan todos los clientes cuya cuota de tráfico está agotada o cuyo plazo ha expirado. Esta acción no se puede deshacer.». Notificación: «Clientes agotados eliminados: {count}».

#### Exportación, importación y eliminación de clientes sin vinculación

Cuando no hay nada seleccionado, en el menú **Más** de la página **Clientes** hay tres operaciones disponibles.

**Exportar clientes** (`GET /clients/export`) abre un visor con la lista JSON de todos los clientes en el formato `{client, inboundIds}` con botones de copia y descarga (archivo `clients-export.json`). **Importar clientes** (`POST /clients/import`) abre un editor en el que se pega ese mismo JSON y se pulsa **Import**: los clientes con `inboundIds` se crean y vinculan a los inbound; los clientes sin vinculaciones se restauran como registros «vacíos» independientes; los email ya existentes **nunca se sobreescriben** — se incluyen en la lista de omitidos. Notificaciones: «{count} clients imported», «{ok} imported, {failed} skipped».

**Eliminar clientes sin vinculación** (`POST /clients/delOrphans`) — operación peligrosa: elimina a todos los clientes que no están vinculados a ningún inbound, junto con su registro de tráfico, el registro de IP y los enlaces externos. Confirmación: «Delete clients without an inbound?», «Removes every client that is not attached to any inbound, along with its traffic record. This cannot be undone.». Notificación: «{count} unattached clients deleted». La acción es irreversible.

### 8.5. Búsqueda, filtros y ordenación

Encima de la lista hay un campo de búsqueda («Buscar email, comentario, sub ID, UUID, contraseña, auth…») — busca por email, comentario, subId, UUID, contraseña y auth. Contador de resultados: «Mostrando {shown} de {total}».

La lista de clientes se actualiza automáticamente: el panel solicita la página actual cada pocos segundos, por lo que los clientes recién conectados y los cambios en el orden de clasificación aparecen sin necesidad de actualizar manualmente (el indicador de carga no parpadea durante la consulta en segundo plano).

El panel **Filtrar clientes** permite seleccionar por estado (categoría), protocolo, inbound vinculado, rango de fecha de expiración, rango de tráfico consumido, presencia de renovación automática (**Con/Sin**), presencia de ID de Telegram y comentario, así como por grupo. En paneles con nodos aparece un selector múltiple **Nodos**: se puede limitar la lista a los clientes de los nodos seleccionados; un elemento separado **Panel local** filtra los clientes de inbound sin vinculación a un nodo (el filtro solo aparece si hay nodos). Ordenación: **Más antiguos/recientes primero**, **Actualizados recientemente**, **Conectados recientemente**, **Email A→Z / Z→A**, **Mayor tráfico**, **Mayor saldo**, **Próximos a expirar**.

### 8.6. Iconos y estados

Prioridad de estados: agotado/expirado → inactivo → próximo a expirar → activo.

- **En línea** / **Sin conexión** — cliente con una conexión activa (presente en la lista en línea actual) y **habilitado**. La lista en línea se actualiza con solicitudes independientes (`/onlines`, `/onlinesByGuid`).
- **Agotado** (depleted) — cuota consumida (`up + down >= totalGB`) **o** plazo expirado (`expiryTime <= ahora`). Ese cliente se deshabilita automáticamente y queda bajo la acción **Eliminar agotados**.
- **Próximo a expirar / agotarse** (expiring) — cliente habilitado al que le queda menos del intervalo umbral hasta la expiración del plazo **o** menos del volumen umbral hasta agotar la cuota (los umbrales se configuran en los ajustes del panel). No se aplica si el cliente ya está agotado/deshabilitado.
- **Inactivo** (deactive) — cliente con `enable = false` (deshabilitado manualmente o por una tarea en segundo plano).
- **Activo** (active) — habilitado, no agotado, plazo no expirado y aún lejos de los umbrales.

---

## 9. Grupos de clientes

> Esta es una función específica de este fork de 3X-UI. En el proyecto original 3x-ui (MHSanaei) no existe el concepto de «grupo de clientes» — aquí se han añadido una tabla separada de grupos, la página **Grupos** en el menú del panel y los métodos de API correspondientes. Si migra la configuración al 3x-ui original, la etiqueta de grupo simplemente no se tendrá en cuenta en ningún lugar.

### 9.1. Qué es un grupo de clientes y para qué sirve

Un **grupo** es una etiqueta lógica con nombre (label) que puede asignarse a uno o varios clientes. No crea un nuevo método de conexión y no es ni un inbound ni un nodo — es puramente una etiqueta organizativa que facilita filtrar y procesar clientes de forma masiva.

La idea clave del modelo de clientes en este fork: **el cliente es una entidad de primer nivel, identificada por email** (el campo `email` en la tabla `clients` tiene un índice único). El mismo cliente (un email con las mismas credenciales) puede pertenecer simultáneamente a varios inbound e incluso a varios nodos, incluso con protocolos distintos. La etiqueta de grupo se almacena **una sola vez por cliente**, por lo que se aplica automáticamente a todas sus asociaciones con inbound a la vez.

La etiqueta de grupo es una etiqueta lógica de agrupación:

| Capa | Dónde se almacena | Campo |
|------|-------------------|-------|
| Registro del cliente (BD) | tabla `clients` | `group_name` (por defecto cadena vacía `''`) |
| Catálogo de grupos (BD) | tabla `client_groups` | `name` (índice único, no vacío) |
| Configuración del inbound (Xray) | JSON `settings.clients[].group` | se replica en cada objeto de cliente de cada inbound al que pertenece el cliente |

Para qué sirve en la práctica:

- **Un cliente en varios inbound/nodos.** Si un cliente «se vende» como acceso a varios inbound a la vez (por ejemplo, distintos protocolos o distintos nodos), el grupo permite gestionarlo como una unidad: reiniciar el tráfico, eliminar, renombrar la etiqueta — con una sola operación sobre todos sus inbound.
- **Operaciones masivas y filtrado.** En la página **Clientes** la lista puede filtrarse por grupo; en la página **Grupos** están disponibles acciones masivas sobre todos los miembros del grupo.
- **Organización de un gran número de clientes.** Etiquetas como `vip`, `trial`, `team-A` permiten clasificar miles de clientes en categorías lógicas sin necesidad de crear inbound adicionales.

### 9.2. Relación del grupo con clientes, inbound, nodos y protocolos

Este es el apartado más importante para comprender el funcionamiento, ya que la sincronización de la etiqueta no es trivial.

**El grupo describe al cliente, no al inbound.** La etiqueta vive en el registro del cliente (`clients.group_name`). Cuando un cliente está asociado a varios inbound, ante cualquier cambio de grupo el panel recorre **todos** los inbound en los que está ese cliente y actualiza/elimina el campo `group` dentro de su configuración de Xray (`settings.clients[]`). Técnicamente esto funciona así: a partir del email del cliente se localizan todos los inbound en los que está, y luego en el JSON de configuración de cada uno de esos inbound se modifica el objeto de cliente con ese email. Por tanto:

- El grupo **no depende del protocolo.** Un mismo email puede ser cliente VLESS en un inbound y cliente Hysteria en otro — la etiqueta de grupo es la misma y se aplicará a ambos (las credenciales de cada protocolo son propias y se almacenan por separado).
- El grupo **abarca nodos.** Los inbound que pertenecen a nodos se distinguen de los inbound del panel principal por el campo `nodeId` (en los inbound del panel principal es `null`/`0`). La etiqueta de grupo se propaga a los objetos de cliente en los inbound independientemente de si es un inbound principal o de nodo — siempre que el cliente con ese email esté presente.

**La etiqueta de grupo es resistente a la sincronización con nodos y a la reconstrucción de configuraciones.** Este comportamiento está diseñado expresamente:

- Cuando un nodo envía un snapshot de tráfico, sus datos **no sobrescriben** los campos `group_name` y `comment` del cliente en la BD del panel. El grupo y el comentario se consideran campos «locales del panel» — el nodo no los gestiona.
- Al reconstruir la configuración de un inbound, un valor vacío de `group` en los datos entrantes **no borra** la etiqueta ya guardada. El grupo se gestiona exclusivamente a través de la página **Grupos** (no mediante la edición de la configuración de Xray del inbound), por lo que un «grupo vacío» en una reconstrucción normal se interpreta como «no tocar», no como «borrar».

Conclusión práctica: las únicas operaciones que **intencionalmente borran** la etiqueta son eliminar el grupo y eliminar explícitamente un cliente del grupo (véase más abajo). La edición habitual del inbound o la sincronización en segundo plano con el nodo no harán desaparecer el grupo.

### 9.3. Catálogo de grupos y grupos «vacíos»

La lista de grupos que se muestra en la página se forma combinando dos fuentes:

1. **Grupos derivados (derived)** — todos los valores no vacíos de `group_name` que realmente aparecen en los clientes, con el recuento de clientes.
2. **Grupos almacenados (stored)** — registros de la tabla `client_groups`.

Esta unión produce un efecto importante: un grupo puede existir **sin ningún cliente**. Este tipo de grupo se crea con el botón «Añadir grupo» (registro en `client_groups`) y aparece en la lista con el contador `0`. Estos registros son los llamados **grupos vacíos**. La lista siempre está ordenada por nombre sin distinción de mayúsculas y minúsculas.

Contadores de resumen en la página:

| Campo | Qué muestra |
|-------|-------------|
| Total de grupos | Número total de grupos (almacenados y derivados juntos). |
| Clientes con grupo | Cuántos clientes tienen una etiqueta de grupo no vacía. |
| Grupos vacíos | Cuántos grupos existen sin clientes (contador `0`). |
| Clientes en el grupo | Número de clientes en un grupo concreto (columna de la tabla). |

### 9.4. Campos y columnas del grupo

El registro de grupo en la tabla `client_groups` contiene:

| Campo | Tipo | Por defecto | Descripción |
|-------|------|-------------|-------------|
| `Id` | int | autoincremento | Clave primaria del registro de grupo. |
| `Name` | string | — (obligatorio) | Nombre del grupo. Índice único, no puede estar vacío. En la UI — columna **Nombre**. |
| `CreatedAt` | int64 (ms) | momento de creación | Momento de creación del registro de grupo. |
| `UpdatedAt` | int64 (ms) | momento de modificación | Momento de la última modificación. |

En la tabla de la página se muestran al menos las columnas **Nombre** y **Clientes en el grupo**, además de los botones de acción (véase más abajo).

### 9.5. Creación de un grupo

Botón **Añadir grupo**.

Pasos:
1. Haga clic en **Añadir grupo**.
2. Introduzca el nombre del grupo.
3. Confirme.

Comportamiento del backend (`POST /panel/api/clients/groups/create`, cuerpo `{"name": "..."}`):
- El nombre se recorta de espacios en los extremos. Un nombre vacío se rechaza con el error «group name is required».
- Si ya existe un grupo con ese nombre — error «group already exists».
- Si tiene éxito, se crea un registro en `client_groups` (inicialmente sin clientes — es un grupo vacío).

Mensaje de éxito: **«Grupo «{name}» creado.»**

**Ejemplo: crear un grupo vacío a través de la API.** Prepare un conjunto de etiquetas de antemano, antes de añadir clientes:

```bash
curl -s 'https://panel.example.com:2053/panel/api/clients/groups/create' \
  -H 'Content-Type: application/json' \
  -b cookie.txt \
  -d '{"name": "vip"}'
```

Respuesta en caso de éxito:

```json
{ "success": true, "msg": "Группа «vip» создана.", "obj": null }
```

Una llamada repetida con el mismo nombre devolverá `"success": false` y el mensaje `group already exists`.

> Crear un grupo vacío de antemano es conveniente cuando desea preparar un conjunto de etiquetas y luego añadirles clientes a través de «Añadir clientes…».

### 9.6. Cambio de nombre de un grupo

Botón **Renombrar**, título del diálogo — **«Renombrar {name}»**.

Comportamiento (`POST /panel/api/clients/groups/rename`, cuerpo `{"oldName": "...", "newName": "..."}`):
- Ambos nombres se recortan de espacios. Nombre antiguo vacío — error «old group name is required», nombre nuevo vacío — «new group name is required».
- Si el nombre nuevo coincide con el antiguo — no se realiza ninguna acción (0 clientes afectados).
- De lo contrario, el cambio de nombre se ejecuta de forma atómica:
  - el registro en `client_groups` se renombra;
  - en todos los clientes con `group_name = oldName` el campo se actualiza a `newName`;
  - en **todos los inbound** en los que están los clientes afectados (incluidos los de nodos), en la configuración de Xray el valor de `group` se cambia del antiguo al nuevo.
- Tras el cambio de nombre, el panel marca Xray como pendiente de reinicio y envía una notificación de cambio de clientes.

Mensajes:
- Éxito: **«Grupo renombrado para {count} cliente(s).»**
- Conflicto de nombres en la UI: **«Ya existe un grupo con el nombre «{name}».»**

### 9.7. Añadir clientes a un grupo

Botón **Añadir clientes…**, título — **«Añadir clientes al grupo «{name}»»**.

Texto literal de la ayuda en el diálogo:

> «Seleccione los clientes que desea añadir a este grupo. Las asociaciones existentes con inbound se conservan; solo cambia la etiqueta de grupo. Los clientes que ya pertenecen a este grupo no se muestran.»

Si no hay nadie a quien añadir, se muestra **«No hay otros clientes para añadir.»**

Comportamiento (`POST /panel/api/clients/groups/bulkAdd`, cuerpo `{"emails": [...], "group": "..."}`):
- El nombre del grupo es obligatorio (de lo contrario, error «group name is required»); lista de emails vacía — la operación no hace nada.
- Si dicho grupo todavía no existe ni en `client_groups` ni entre los derivados — se creará automáticamente.
- Para los emails seleccionados se establece `group_name = group` en los clientes; **las asociaciones de los clientes con inbound no cambian** — solo se modifica la etiqueta. Luego en todos los inbound de esos clientes se establece el campo `group`.
- Se devuelve el número de registros de clientes afectados; Xray se marca para reinicio.

Mensaje de éxito: **«{count} cliente(s) añadido(s) a {name}.»**

**Ejemplo: etiquetar varios clientes con un grupo en una sola solicitud.** Los clientes se especifican por email; las asociaciones con inbound no cambian:

```bash
curl -s 'https://panel.example.com:2053/panel/api/clients/groups/bulkAdd' \
  -H 'Content-Type: application/json' \
  -b cookie.txt \
  -d '{"emails": ["alice@example.com", "bob@example.com"], "group": "vip"}'
```

Si el grupo `vip` aún no existe, se creará automáticamente. Tras la solicitud, en el registro de estos clientes se establecerá `group_name = "vip"`, y en la configuración de Xray de cada uno de sus inbound el objeto de cliente recibirá el campo `"group": "vip"`:

```json
{ "id": "6f1b...", "email": "alice@example.com", "group": "vip", "enable": true }
```

### 9.8. Eliminación de clientes de un grupo (sin eliminar los propios clientes)

Botón **Eliminar clientes…**, título — **«Eliminar clientes del grupo «{name}»»**.

Texto literal de la ayuda:

> «Seleccione los miembros que desea eliminar de este grupo. Los propios clientes se conservan (utilice «Eliminar clientes del grupo» para la eliminación completa).»

Comportamiento (`POST /panel/api/clients/groups/bulkRemove`, cuerpo `{"emails": [...]}`): técnicamente es lo mismo que «Añadir al grupo» con un grupo vacío. En los clientes seleccionados se borra `group_name`, y en sus inbound se elimina el campo `group` de la configuración de Xray. Los propios clientes y sus asociaciones con inbound se conservan.

Mensaje de éxito: **«{count} cliente(s) eliminado(s) de {name}.»**

### 9.9. Reinicio del tráfico del grupo

Botón **Reiniciar tráfico**.

Diálogo de confirmación:
- Título: **«¿Reiniciar el tráfico del grupo {name}?»**
- Texto: **«Esto pondrá a cero up/down de los {count} cliente(s) de este grupo.»**

Comportamiento: para todos los emails de los miembros del grupo se ponen a cero `up` y `down` en la tabla de tráfico y el campo `enable` se establece en `true` (el cliente se activa). La operación se ejecuta en lotes dentro de una transacción.

Mensaje de éxito: **«Tráfico reiniciado para {count} cliente(s).»**

### 9.10. Eliminación del grupo y eliminación de clientes del grupo

En la página existen **dos operaciones de eliminación fundamentalmente distintas** — es fácil confundirlas, por lo que la diferencia es crítica.

#### 9.10.1. Eliminar el grupo (conservar los clientes)

Botón **«Eliminar el grupo (conservar los clientes)»**.

Diálogo:
- Título: **«¿Eliminar el grupo {name}?»**
- Texto: **«Esto elimina el grupo y borra su etiqueta en {count} cliente(s). Los propios clientes no se eliminan.»**

Comportamiento (`POST /panel/api/clients/groups/delete`, cuerpo `{"name": "..."}`): el registro del grupo se elimina de `client_groups`, en todos sus clientes se borra `group_name`, y de sus inbound se elimina el campo `group`. **Los clientes, sus conexiones y su tráfico se conservan.** Xray se marca para reinicio.

Mensaje de éxito: **«Grupo borrado para {count} cliente(s).»**

#### 9.10.2. Eliminar los clientes del grupo (eliminación completa)

Botón **«Eliminar clientes del grupo»**.

Diálogo:
- Título: **«¿Eliminar todos los clientes de {name}?»**
- Texto: **«Esto elimina de forma irreversible {count} cliente(s) junto con sus registros de tráfico. La etiqueta de grupo también se borra. Esta acción no se puede deshacer.»**

Esta es una operación destructiva: elimina los propios clientes (mediante eliminación masiva por email, endpoint `POST /panel/api/clients/bulkDel`), incluyendo sus registros de tráfico, y por tanto los elimina de todos los inbound.

Mensajes:
- Éxito: **«{count} cliente(s) eliminado(s).»**
- Resultado parcial: **«{ok} eliminado(s), {failed} omitido(s)»**

> Si el grupo está vacío, las acciones sobre sus miembros no están disponibles — se muestra **«Este grupo aún no tiene clientes.»**

### 9.11. Relación con la página «Clientes»

La etiqueta de grupo es visible y se utiliza también fuera de la página **Grupos**:

- En el registro compacto del cliente existe el campo `group`, por lo que en la lista de clientes se muestra la pertenencia al grupo.
- La lista de clientes (`/panel/api/clients/list/paged`) acepta el parámetro de filtro `group`: puede pasarse un nombre o varios separados por comas. La comparación se realiza con lógica «O» dentro del campo, sin distinción de mayúsculas y minúsculas. Caso especial: un elemento vacío en la lista de grupos del filtro significa «clientes sin grupo» (aquellos cuyo `group` está vacío).
- En la respuesta de la página de clientes se devuelve el array `groups` — lista completa de nombres de los grupos existentes, para que la UI pueda construir el menú desplegable de filtro.

**Ejemplo: filtrar clientes por grupos.** La solicitud devuelve solo los clientes con las etiquetas `vip` o `trial` (varios nombres separados por comas, lógica «O»):

```
GET /panel/api/clients/list/paged?group=vip,trial
```

Para obtener los clientes **sin** grupo, pase un elemento vacío en la lista — por ejemplo, el valor de filtro `group=` (cadena vacía) o `group=vip,` (etiqueta `vip` más clientes sin grupo).

### 9.12. Resumen de endpoints de API

Todas las rutas de grupos están montadas bajo `/panel/api/clients`:

| Método y ruta | Propósito | Cuerpo de la solicitud |
|---------------|-----------|------------------------|
| `GET /panel/api/clients/groups` | Lista de grupos con contadores de clientes | — |
| `GET /panel/api/clients/groups/:name/emails` | Emails de todos los miembros del grupo (ordenados por email) | — |
| `POST /panel/api/clients/groups/create` | Crear un grupo vacío | `{"name"}` |
| `POST /panel/api/clients/groups/rename` | Renombrar un grupo | `{"oldName","newName"}` |
| `POST /panel/api/clients/groups/delete` | Eliminar el grupo conservando los clientes (borrar la etiqueta) | `{"name"}` |
| `POST /panel/api/clients/groups/bulkAdd` | Añadir clientes a un grupo (por email) | `{"emails":[...],"group"}` |
| `POST /panel/api/clients/groups/bulkRemove` | Quitar clientes de un grupo (borrar la etiqueta) | `{"emails":[...]}` |
| `POST /panel/api/clients/bulkDel` | Eliminación completa de clientes (utilizado por «Eliminar clientes del grupo») | `{"emails":[...],"keepTraffic"}` |

**Ejemplo: escenario típico del ciclo de vida de un grupo a través de la API.**

```bash
# 1. Crear la etiqueta trial
curl -s .../panel/api/clients/groups/create   -d '{"name":"trial"}'

# 2. Asignarla a dos clientes
curl -s .../panel/api/clients/groups/bulkAdd  -d '{"emails":["u1@example.com","u2@example.com"],"group":"trial"}'

# 3. Reiniciar el tráfico de todos los miembros (por email de /groups/trial/emails)
curl -s .../panel/api/clients/groups/bulkRemove -d '{"emails":["u2@example.com"]}'

# 4. Eliminar el grupo pero conservar los clientes (solo borrar la etiqueta)
curl -s .../panel/api/clients/groups/delete   -d '{"name":"trial"}'
```

El paso 4 elimina el registro del grupo y borra `group_name` en sus clientes, pero los propios clientes, sus conexiones y su tráfico permanecen. Para la eliminación irreversible de los propios clientes se utiliza `bulkDel` en su lugar.

Las operaciones que modifican la etiqueta en los clientes (`rename`, `delete`, `bulkAdd`, `bulkRemove`) marcan Xray como pendiente de reinicio y envían una notificación de cambio de clientes.

### 9.13. Tráfico por grupo

Novedad de la versión 3.3.0: en la sección **Grupos** (página «Clientes», pestaña de gestión de grupos) la tabla de grupos ahora muestra no solo el número de clientes en cada grupo, sino también el tráfico total consumido por el grupo. La columna lleva el título **«Tráfico utilizado»**.

#### Qué muestra la columna

Para cada fila de grupo se muestra la suma del tráfico de todos los clientes pertenecientes a ese grupo — es decir, la suma de `up + down` (tráfico enviado + recibido) de todos sus miembros. Esto da una respuesta rápida a la pregunta «cuánto ha descargado/subido en total el grupo», sin necesidad de abrir los clientes uno a uno y sumar manualmente.

Junto a la tabla de grupos también se muestran:

| Columna | Qué significa |
|---------|---------------|
| Nombre | Nombre del grupo |
| Clientes | Cuántos clientes están etiquetados con este grupo (antes la columna se llamaba «Clientes en el grupo») |
| Enviado | `up` total (tráfico enviado) de todos los clientes del grupo |
| Recibido | `down` total (tráfico recibido) de todos los clientes del grupo |
| Tráfico utilizado | `up + down` total de todos los clientes del grupo |

El tráfico enviado y recibido se muestran en columnas separadas **Enviado** y **Recibido**, y la columna **Tráfico utilizado** muestra su suma. La columna del número de clientes se llama simplemente **Clientes**.

El resumen sobre la tabla muestra adicionalmente agregados de todos los grupos — **«Total de grupos»** y **«Clientes con grupo»**, y el tráfico total se divide en dos tarjetas: **«Total enviado / recibido»** (con flechas arriba/abajo — tráfico enviado y recibido por separado de todos los grupos) y **«Tráfico total»** (con icono de diagrama — su suma total).

#### Cómo se calcula

El cálculo se realiza con una sola consulta SQL a la tabla de clientes con una unión (`LEFT JOIN`) a la tabla de contabilidad de tráfico:

- por el campo de etiqueta de grupo (`group_name`) los clientes se agrupan y se cuenta su número — esto es «Clientes en el grupo»;
- el tráfico se toma como la suma de `up + down` de la tabla unida `client_traffics`. Es decir, se suman tanto los bytes enviados (`up`) como los recibidos (`down`) de cada cliente;
- dado que el email es único tanto en la tabla de clientes como en la tabla de tráfico, la unión no duplica el tráfico de un cliente.

Particularidades de los valores:

- **Los clientes sin registro de tráfico** se contabilizan en el contador de miembros, pero aportan 0 a la suma, por lo que un grupo recién creado muestra tráfico `0`.
- **Los grupos vacíos** (creados pero sin clientes) también están presentes en la lista con contador y tráfico cero: además de los grupos «derivados» de las etiquetas de los clientes, en el resultado se mezclan los grupos explícitamente almacenados, y luego la lista se ordena por nombre sin distinción de mayúsculas y minúsculas.
- Los clientes sin etiqueta de grupo (`group_name` vacío) no se incluyen en el cálculo.

#### Acciones relacionadas

Desde la tabla de grupos siguen estando disponibles las acciones sobre el grupo completo, entre ellas **«Reiniciar tráfico»** — pone a cero `up`/`down` de todos los clientes del grupo seleccionado. Tras dicho reinicio, la columna «Tráfico utilizado» para ese grupo muestra `0`.

---

## 10. Suscripciones (Subscription)

Una suscripción (subscription) es un mecanismo que permite entregar al cliente un único enlace permanente (URL) mediante el cual la aplicación VPN descarga y actualiza periódicamente el conjunto completo de configuraciones. En lugar de enviar manualmente al usuario un enlace separado para cada inbound, se le proporciona una dirección única del tipo `https://dominio:puerto/sub/<subId>`. A través de esta dirección, el panel ensambla al vuelo todas las configuraciones vinculadas a ese cliente y las devuelve en el formato que el cliente necesita. Cuando cambia la configuración en el servidor (nueva dirección, rotación de claves Reality, adición de inbound), el cliente recibe la configuración actualizada en la próxima actualización automática, sin necesidad de ninguna acción por parte del usuario.

La suscripción es atendida por un servidor HTTP/HTTPS separado dentro del panel, que se inicia de forma independiente del panel web y escucha en su propio puerto. Esto se hace por razones de seguridad: el puerto de suscripción puede abrirse al exterior sin abrir el puerto del propio panel.

### 10.1. Qué es subId y cómo se forma el enlace

Cada cliente en un inbound tiene el campo `subId` (en la interfaz: «ID de suscripción»). Este valor es la clave de la suscripción: el panel busca en todos los inbound los clientes cuyo `subId` coincida con el solicitado y combina sus configuraciones en una única respuesta.

- Si varios clientes (en uno o en distintos inbound) tienen el mismo `subId`, sus configuraciones se incluirán en una sola suscripción. Esta es la forma estándar de entregar a un usuario varios servidores/protocolos mediante un único enlace.

**Ejemplo: un usuario — dos servidores con un solo enlace.** Supongamos que hay dos inbound (VLESS en el servidor A y Trojan en el servidor B). Para entregar al usuario ambas configuraciones con un solo enlace, asigne el mismo `subId` a ambos clientes:

```
Inbound 1 (VLESS):  email = ivan@vpn,  subId = ivan2025
Inbound 2 (Trojan): email = ivan@vpn,  subId = ivan2025
```

Entonces en la dirección `https://sub.example.com:2096/sub/ivan2025` el panel devolverá ambas configuraciones a la vez. Si más adelante agrega un tercer inbound con el mismo `subId`, aparecerá para el usuario en la próxima actualización automática de la suscripción, sin necesidad de enviar un nuevo enlace.
- Si el campo `subId` del cliente está vacío, no es posible compartir un enlace de acceso general. En la interfaz esto se indica con el mensaje: «Este cliente no tiene subId, el enlace de acceso compartido no está disponible.»

#### Enlaces externos y suscripciones del cliente (pestaña «Links»)

En el formulario del cliente hay una pestaña **«Links»**, donde para un cliente individual se pueden adjuntar fuentes adicionales de configuraciones que se mezclan específicamente en su suscripción (formatos RAW, JSON y Clash):

- **Add External Link** — enlace de compartición externo (`vless://`, `trojan://`, `ss://`, etc.). Se agrega a la respuesta tal cual, y para JSON/Clash se analiza adicionalmente como configuración.
- **Add External Subscription** — dirección de una suscripción externa. El panel la descarga por sí solo (con caché y un tiempo de espera corto) e incorpora las configuraciones obtenidas a la lista general del cliente.

Esto es útil para entregar al cliente servidores adicionales además de sus inbound mediante el mismo enlace único. Si la respuesta de la suscripción remota es demasiado grande, ya no se trunca silenciosamente: el panel devuelve un error y sigue utilizando el último valor en caché con éxito.
- El valor de `subId` no puede establecerse de forma arbitraria: al guardar se verifica que no contenga espacios, símbolos `/`, `\` ni caracteres de control. El mensaje de validación correspondiente es: «El ID de suscripción no puede contener espacios, '/', '\' ni caracteres de control».

El enlace final se construye como `<base>/<subPath>/<subId>` (véase la sección sobre la configuración del servidor de suscripciones y el campo «URI de proxy inverso»). Si no se encuentra ningún cliente por `subId` (el cliente fue eliminado, el `subId` no existe), el servidor devuelve HTTP 404 sin cuerpo. En caso de error interno — HTTP 500. Las aplicaciones VPN solo interpretan el código de respuesta, por lo que el cuerpo del error se deja intencionalmente vacío.

#### Orden de los enlaces de inbound en la suscripción

Cada inbound tiene el campo **«Orden en suscripción»** (`subSortIndex`) — un número desde 1 que establece la posición de los enlaces de ese inbound en la respuesta de la suscripción. Los valores menores aparecen primero; con valores iguales se conserva el orden de creación original (por id). El orden se aplica a todos los formatos de respuesta: texto sin formato, página de suscripción, JSON y Clash. Este campo no afecta el orden de los inbounds en el propio panel.

El campo se edita en el formulario de inbound junto a la configuración de la dirección en el enlace (share address) y se sincroniza con los nodos según las reglas habituales. Si al menos un inbound tiene un orden distinto de 1, en la lista de Inbounds aparece una columna compacta **«Orden»**.

### 10.2. Configuración del servidor de suscripciones

Todos los parámetros de suscripción se encuentran en la sección de configuración del panel en la pestaña **«Suscripción»**. A continuación se describe cada parámetro; entre paréntesis se indica la clave interna de configuración y el valor predeterminado.

La propia sección está dividida en pestañas: **«Configuración del panel»**, **«Información»**, **«Perfil»**, **«Certificados»**, **«Happ»** y **«Clash / Mihomo»**. Los campos de título de suscripción, URL de soporte, página de perfil, anuncio y directorio de tema se encuentran en la pestaña «Perfil»; las reglas de enrutamiento de Happ y Clash/Mihomo, en las pestañas correspondientes; el intervalo de actualización de suscripción, en la pestaña «Información».

#### Parámetros principales

| Campo (UI) | Clave | Valor predeterminado | Descripción |
|---|---|---|---|
| Activar suscripción | `subEnable` | `true` (activado) | Inicia un servidor de suscripciones separado. Descripción: «Función de suscripción con configuración independiente». Si está desactivado, el servidor de suscripciones no se inicia y ningún enlace funciona. |
| IP de escucha | `subListen` | vacío | Dirección IP en la que el servidor de suscripciones acepta conexiones. Descripción: «Déjelo vacío por defecto para escuchar en todas las direcciones IP». |
| Puerto de suscripción | `subPort` | `2096` | Puerto TCP del servidor de suscripciones. Descripción: «El número de puerto para el servicio de suscripción no debe estar en uso en el servidor» — el puerto debe estar libre y no entrar en conflicto con el panel o Xray. |
| Ruta URI | `subPath` | `/sub/` | Ruta en la que se sirven las suscripciones normales. Descripción: «Debe comenzar con '/' y terminar con '/'». |
| Dominio de escucha | `subDomain` | vacío | Dominio para el que se permite el acceso a la suscripción (validación de Host). Descripción: «Déjelo vacío por defecto para escuchar en todos los dominios y direcciones IP». Si se establece, las solicitudes con otro Host son rechazadas. |

**Nota de seguridad importante:** la ruta predeterminada `/sub/` (y `/json/` para JSON) es ampliamente conocida y fácilmente adivinable. El panel muestra una advertencia: «La ruta de suscripción predeterminada "/sub/" es ampliamente conocida — cámbiela.» y un mensaje similar para JSON. Se recomienda establecer una ruta personalizada no obvia.

#### TLS / certificado

| Campo (UI) | Clave | Predeterminado | Descripción |
|---|---|---|---|
| Ruta al archivo de clave pública del certificado de suscripción | `subCertFile` | vacío | Ruta completa al archivo de certificado (`.crt`/`fullchain`). Descripción: «Introduzca la ruta completa que comience con '/'». |
| Ruta al archivo de clave privada del certificado de suscripción | `subKeyFile` | vacío | Ruta completa al archivo de clave privada. Descripción: «Introduzca la ruta completa que comience con '/'». |

Si se establecen ambas rutas y el certificado se carga correctamente, el servidor de suscripciones funciona en **HTTPS**. Si los campos están vacíos o el certificado no se pudo leer, el servidor regresa a **HTTP** (el error se registra en el log). La presencia de TLS válido también influye en la formación de la URL base: con el puerto 443 con TLS y el puerto 80 sin TLS, el número de puerto se omite en el enlace.

#### Intervalo de actualización

| Campo (UI) | Clave | Predeterminado | Descripción |
|---|---|---|---|
| Intervalos de actualización de suscripción | `subUpdates` | `12` | Con qué frecuencia (en horas) la aplicación cliente debe volver a solicitar la suscripción. Descripción: «Intervalo entre actualizaciones en la aplicación cliente (en horas)». |

El valor se transmite al cliente en el encabezado HTTP `Profile-Update-Interval`; los clientes modernos lo utilizan como período de actualización automática de la configuración.

#### Formato e información en la respuesta

| Campo (UI) | Clave | Predeterminado | Descripción |
|---|---|---|---|
| Codificar | `subEncrypt` | `true` | Descripción: «Cifrar las configuraciones devueltas en la suscripción». Técnicamente no es cifrado, sino **codificación Base64** de todo el cuerpo de la suscripción normal (el formato que espera la mayoría de los clientes). Si está desactivado, los enlaces se devuelven en texto sin formato, uno por línea. |
| Mostrar información de uso | `subShowInfo` | `true` | Descripción: «Mostrar el tráfico restante y la fecha de vencimiento después del nombre de la configuración». Cuando está activado, se añaden marcadores de tráfico restante (📊) y fecha de vencimiento (por ejemplo, `5D,3H⏳`) al nombre (remark) de cada configuración; cuando el cliente ha vencido o no está disponible se muestra `⛔️N/A`. |
| Incluir Email en el nombre | `subEmailInRemark` | `true` | Descripción: «Incluir el email del cliente en el nombre del perfil de suscripción.». Agrega el email del cliente al remark del perfil. |

#### Plantilla de remark (Remark Template)

El nombre mostrado (remark) de cada configuración en la suscripción se forma según la **plantilla de remark** — el campo **«Plantilla de nota»** (`remarkTemplate`) en la pestaña **«Información»** de la configuración de suscripción. El antiguo constructor del modelo de nota (selección separada de partes de inbound/email/proxy externo y símbolo separador) ha sido eliminado de la interfaz; ahora se escribe un formato de nombre arbitrario y se insertan variables en él. El valor predeterminado es `{{INBOUND}}-{{EMAIL}}|📊{{TRAFFIC_LEFT}}|⏳{{DAYS_LEFT}}D` (es decir, por defecto el nombre del perfil contiene el email del cliente). Si el campo se deja vacío, se aplica el modelo de remark anterior (no configurable desde la interfaz).

Las variables están agrupadas en secciones **Client**, **Traffic** y **Time & status** y se muestran junto al campo como chips clicables `{{VAR}}` con descripción emergente al pasar el cursor; al hacer clic se inserta el token en la plantilla, y hay una vista previa en vivo. Cada variable se sustituye individualmente para cada cliente en el momento de generar la suscripción. También se acepta la notación simplificada entre llaves simples (`{DATA_LEFT}`, `{EXPIRE_DATE}`, `{PROTOCOL}`, `{TRANSPORT}`, etc.) — el panel la convierte automáticamente al formato interno `{{...}}`.

Variables disponibles:

- **Identificación del cliente:** `{{EMAIL}}`, `{{INBOUND}}` (remark del propio inbound), `{{HOST}}` (remark del host), `{{ID}}` (UUID), `{{SHORT_ID}}` (primeros 8 caracteres del UUID), `{{SUB_ID}}`, `{{COMMENT}}`, `{{TELEGRAM_ID}}`, `{{PROTOCOL}}`, `{{TRANSPORT}}`.
- **Tráfico:** `{{TRAFFIC_USED}}`, `{{TRAFFIC_LEFT}}`, `{{TRAFFIC_TOTAL}}` (y sus variantes `*_BYTES` en bytes exactos), `{{UP}}`, `{{DOWN}}`, `{{USAGE_PERCENTAGE}}`.
- **Plazo y estado:** `{{DAYS_LEFT}}`, `{{TIME_LEFT}}`, `{{EXPIRE_DATE}}` (`AAAA-MM-DD`), `{{JALALI_EXPIRE_DATE}}` (fecha en calendario Jalali), `{{EXPIRE_UNIX}}`, `{{CREATED_UNIX}}`, `{{RESET_DAYS}}`, `{{STATUS}}` (active / expired / disabled / depleted), `{{STATUS_EMOJI}}`.
- **Conexión (Connection):** `{{PROTOCOL}}` — protocolo (VLESS, VMess, Trojan, etc.), `{{TRANSPORT}}` — red de transporte (tcp, ws, grpc, etc.), `{{SECURITY}}` — seguridad del transporte (TLS, REALITY, NONE; se muestra en mayúsculas). Al igual que las variables de uso y plazo, estas tres variables solo funcionan en el cuerpo de la suscripción y se eliminan automáticamente del remark en los enlaces mostrados en el panel (QR/«Información») y en la página de información de la suscripción.

La plantilla puede dividirse en segmentos mediante la barra vertical `|`. Un segmento en el que una variable arroja un valor «ilimitado» (`∞`) — por ejemplo `{{TRAFFIC_LEFT}}` o `{{DAYS_LEFT}}` para un cliente sin restricciones — se oculta automáticamente. Además, el bloque de uso de tráfico y plazo se muestra una sola vez, en el primer enlace del cliente, para no duplicarse en cada configuración.

**Ejemplo.** La plantilla `{{EMAIL}}|📊{{TRAFFIC_LEFT}}|⏳{{DAYS_LEFT}}D` para un cliente con 42 GB restantes y 7 días producirá un nombre como `ivan@vpn 📊42.00GB ⏳7D`, y para un cliente ilimitado — simplemente `ivan@vpn` (los segmentos con `∞` se omiten).

En los enlaces mostrados en el panel (código QR y ventanas «Información» en la página «Clientes») y en la página de información de la suscripción, el email del cliente aparece en el nombre del perfil: formato «inbound-host-email» cuando hay un host definido, o «inbound-email» sin host. Los datos de tráfico y plazo (así como las variables del grupo «Conexión») no se sustituyen en estos nombres mostrados — solo funcionan en el cuerpo de la suscripción que recibe la aplicación VPN.

Si la fila de estadísticas de tráfico del cliente quedó «huérfana» tras eliminar y volver a crear el inbound, la variable `{{TRAFFIC_USED}}` (y otros indicadores de uso) ya no muestra `0.00B`: el panel busca adicionalmente las estadísticas por el email del cliente y sustituye el tráfico utilizado correcto.
| Plantilla de remark | `remarkTemplate` | `{{INBOUND}}|📊{{TRAFFIC_LEFT}}|⏳{{DAYS_LEFT}}D` | Plantilla libre del nombre mostrado (remark) de cada configuración con sustitución de variables `{{VAR}}`. Se sustituye individualmente para cada cliente al generar la suscripción. El antiguo constructor del «modelo de nota» (selección de inbound/email/proxy externo y separador) ha sido eliminado de la interfaz y solo se usa como alternativa si el campo se deja vacío. Para más detalles, véase «Plantilla de remark (Remark Template)» a continuación. |

#### Metadatos del perfil (encabezados de respuesta)

Estas cadenas se transmiten al cliente en los encabezados HTTP de respuesta y se muestran en la aplicación VPN como metadatos del perfil. Todas están vacías de forma predeterminada.

| Campo (UI) | Clave | Encabezado | Descripción |
|---|---|---|---|
| Título de suscripción | `subTitle` | `Profile-Title` (en Base64) | «Nombre de la suscripción que el cliente ve en la aplicación VPN». Para Clash también se utiliza como nombre del perfil importado mediante `Content-Disposition`. |
| URL de soporte | `subSupportUrl` | `Support-Url` | «Enlace de soporte técnico que se muestra en la aplicación VPN». |
| URL del perfil | `subProfileUrl` | `Profile-Web-Page-Url` | «Enlace a su sitio web que se muestra en la aplicación VPN». Si no se establece, se usa la URL real de la solicitud de suscripción. |
| Anuncio | `subAnnounce` | `Announce` (en Base64) | «Texto del anuncio que se muestra en la aplicación VPN». |

Además, en cada respuesta se transmite el encabezado `Subscription-Userinfo` con los datos de tráfico agregados del cliente: `upload`, `download`, `total` y `expire` (momento de vencimiento en segundos). La aplicación cliente lo utiliza para mostrar el tráfico restante y la fecha de vencimiento.

#### Enrutamiento (solo para el cliente Happ)

| Campo (UI) | Clave | Predeterminado | Descripción |
|---|---|---|---|
| Activar enrutamiento | `subEnableRouting` | `false` | «Configuración global para activar el enrutamiento en la aplicación VPN. (Solo para Happ)». Se transmite en el encabezado `Routing-Enable`. |
| Reglas de enrutamiento | `subRoutingRules` | vacío | «Reglas de enrutamiento globales para la aplicación VPN. (Solo para Happ)». Se transmiten en el encabezado `Routing`. |

| Ocultar configuración del servidor | `subHideSettings` | `false` | «Ocultar la configuración del servidor en la suscripción (solo para Happ)». Cuando está activado, el cliente Happ oculta la posibilidad de ver y modificar los parámetros del servidor. La opción solo funciona para el cliente Happ. |

#### Enrutamiento Incy (solo para el cliente Incy)

Para la aplicación VPN **Incy**, en la configuración de suscripción hay una pestaña separada **«Incy»** con dos campos: el interruptor **«Activar enrutamiento»** (`subIncyEnableRouting`, desactivado por defecto) y el campo de texto **«Reglas de enrutamiento»** (`subIncyRoutingRules`) con formato `incy://routing/onadd/<base64>`. Cuando el enrutamiento está activado y el campo está completado, esta cadena se añade como línea separada al cuerpo de la suscripción (formato raw) — así el perfil de enrutamiento se entrega al cliente Incy sin interferir con el encabezado `Routing` del cliente Happ. La configuración solo funciona para el cliente Incy.

#### URI de proxy inverso

| Campo (UI) | Clave | Predeterminado | Descripción |
|---|---|---|---|
| URI de proxy inverso | `subURI` | vacío | «Cambiar el URI base de la URL de suscripción para su uso detrás de servidores proxy». |

Si el campo está vacío, el panel construye la dirección base del enlace a partir del dominio y puerto de la suscripción (teniendo en cuenta TLS). Si la suscripción se distribuye a través de un proxy inverso/CDN externo en otro dominio o ruta, en este campo se establece el URI base final, y todos los enlaces se construirán desde él. Existen campos individuales similares para JSON (`subJsonURI`) y Clash (`subClashURI`).

Si solo se establece el `subURI` general y los campos individuales para JSON y Clash se dejan vacíos, los enlaces de esos formatos en la página de suscripción heredan el esquema y el host de `subURI` (no el puerto del servidor sub y `http`) — de modo que coinciden con la dirección del proxy inverso.

**Ejemplo: suscripción detrás de un proxy inverso.** La propia suscripción escucha en `2096`, pero desde el exterior es accesible a través de nginx/CDN en `https://cfg.example.com/u/`. Para que los enlaces en la respuesta se construyan desde la dirección externa y no desde el `dominio:2096` interno, en el campo «Reverse proxy URI» se establece el URI base final:

```
Reverse proxy URI: https://cfg.example.com/u
```

El enlace final tomará la forma `https://cfg.example.com/u/ivan2025`. Para los formatos JSON y Clash, si es necesario, se completan los campos separados `subJsonURI` y `subClashURI` de la misma manera.

### 10.3. Formatos de salida

La suscripción puede entregarse en tres formatos independientes, cada uno con su propio endpoint que puede activarse o desactivarse por separado.

#### Dirección del servidor y nodos en la respuesta

La dirección del servidor en los enlaces de suscripción se sustituye según la misma estrategia de dirección en el enlace que los enlaces normales y los códigos QR en el panel: «listen» — dirección de enlace enrutable, «custom» — dirección personalizada definida por el usuario (`shareAddr`), «node» (por defecto) — dirección del nodo. Para los inbound sin una estrategia establecida explícitamente, la respuesta de suscripción no cambia. Esto permite que un inbound de nodo vinculado a una IP pública específica entregue a los clientes una dirección alcanzable. La estrategia se aplica a los formatos raw, JSON y Clash.

El nombre del nodo (Node) no se añade al nombre (remark) del perfil en la suscripción: en la aplicación cliente solo se muestra el remark del inbound establecido por el administrador, sin el sufijo interno del tipo `@nombre-nodo`. Para distinguir entradas con el mismo nombre en una suscripción multi-nodo, establezca manualmente distintos remarks para cada una o utilice hosts gestionados (Hosts) con sus propios Remark.

Si debido a la desincronización entre nodos el mismo cliente terminó en un inbound JSON de servicio dos veces, la respuesta de suscripción elimina automáticamente tales duplicados por email en los tres formatos, por lo que los perfiles repetidos no aparecen en la respuesta.

#### Hosts gestionados (Hosts)

La sección **Hosts** (elemento del menú lateral; página resumen con el número Total/Enabled/Disabled y la lista) define las sobreescrituras de dirección para los enlaces de suscripción. Para cada inbound se puede añadir uno o varios **hosts** — endpoints que se sustituyen en los enlaces de suscripción entregados al cliente **en lugar de la dirección, el puerto y los parámetros TLS del propio inbound**. Esto es útil para distribuir el tráfico a través de CDN o relay sin modificar el inbound en sí.

Para cada host se establecen:

- **Remark** y descripción (Description), vinculación a un **Inbound** específico, interruptor **Enable** y asignación a nodos (**Nodes**).
- **Address** (vacío — hereda la dirección del inbound) y **Port** (`0` — hereda el puerto del inbound); **Tags** (solo se tienen en cuenta en la suscripción RAW).
- Pestaña **Security** — `same` / `tls` / `none` / `reality` con SNI, huella digital (fingerprint), ALPN, certificado anclado (pinned-cert), `allowInsecure` y ECH.
- Pestaña **Advanced** — encabezado Host, Path, ruta VLESS, Mux, Sockopt, Final Mask y exclusión del host de formatos de suscripción individuales (raw / json / clash).
- Pestaña **Clash (mihomo)** — versión IP, Mihomo X25519, mezcla de hosts (Shuffle host).

Los hosts se ordenan dentro de su inbound y admiten activación, desactivación y eliminación masiva. Los hosts gestionados reemplazan al antiguo array External Proxy.

#### Enlaces normales (SUB) — Base64 / texto sin formato

Formato base, endpoint `subPath` (por defecto `/sub/`). Siempre está activo (cuando la suscripción en general está activada). Devuelve la lista de enlaces Xray (`vless://`, `vmess://`, `trojan://`, `ss://`, etc.) — uno por línea. Con la opción «Codificar» (`subEncrypt`) activada, toda la lista se codifica en Base64; desactivada, se devuelve en texto sin formato. Este formato lo entienden prácticamente todos los clientes (v2rayNG, V2RayTun, Sing-box, NekoBox, Streisand, Shadowrocket, Happ y otros).

**Ejemplo: cuerpo de respuesta con «Codificar» desactivado.** Con `subEncrypt = false`, el endpoint `/sub/` devuelve texto sin formato — un enlace por línea:

```
vless://3c8f...@a.example.com:443?security=reality&...#srvA-ivan
trojan://p4ss@b.example.com:443?security=tls&...#srvB-ivan
```

Con `subEncrypt = true` (predeterminado), la misma lista completa se codifica en Base64 y se devuelve como una sola cadena — exactamente este formato espera la mayoría de los clientes.

#### Suscripción JSON (sing-box y compatibles)

Endpoint `subJsonPath` (por defecto `/json/`), se activa mediante una casilla independiente.

| Campo (UI) | Clave | Predeterminado | Descripción |
|---|---|---|---|
| Suscripción JSON | `subJsonEnable` | `false` | «Activar/desactivar el endpoint JSON de suscripción de forma independiente.». |

Devuelve la configuración JSON completa (formato compatible con sing-box y clientes derivados — Podkop, OpenWRT sing-box, Karing, NekoBox). Para este formato hay parámetros adicionales disponibles (pestaña `subFormats`):

- **Mux** (`subJsonMux`, por defecto vacío) — configuración JSON de multiplexación (Mux) que se inyecta en el outbound de cada flujo de suscripción JSON. «Transmisión de múltiples flujos de datos independientes en una sola conexión.».
- **Final Mask** (`subJsonFinalMask`, por defecto vacío) — «Máscaras finalmask de xray (TCP/UDP) y configuración QUIC añadidas a cada flujo de suscripción JSON. Requiere una versión reciente de xray en el cliente.». Se configura mediante subcampos: «Paquetes» (`packets`), «Longitud» (`length`), «Intervalo» (`interval`), «División máx.» (`maxSplit`), «Ruidos» (`noises`: «Tipo»/`type`, «Paquete»/`packet`, «Retardo (ms)»/`delayMs`, «Aplicar a»/`applyTo`, botón «+ Ruido»), así como «Concurrencia» (`concurrency`), «Concurrencia xudp» (`xudpConcurrency`) y «xudp UDP 443» (`xudpUdp443`).
- **Reglas de enrutamiento** (`subJsonRules`, por defecto vacío) — reglas globales añadidas a la configuración JSON.

#### Suscripción Clash / Mihomo (YAML)

Endpoint `subClashPath` (por defecto `/clash/`), se activa mediante una casilla independiente.

| Campo (UI) | Clave | Predeterminado | Descripción |
|---|---|---|---|
| Suscripción Clash / Mihomo | `subClashEnable` | `false` | Activa la generación de configuración YAML para los clientes Clash y Mihomo. |
| Activar enrutamiento | `subClashEnableRouting` | `false` | «Añadir reglas de enrutamiento globales de Clash/Mihomo a las suscripciones YAML generadas.». |
| Reglas de enrutamiento globales | `subClashRules` | vacío | «Reglas de Clash/Mihomo añadidas al inicio de cada suscripción YAML antes de MATCH,PROXY.». |

La respuesta se devuelve con el tipo `application/yaml; charset=utf-8`. Si se establece el «Título de suscripción» (`subTitle`), también se transmite en el encabezado `Content-Disposition` (`attachment; filename*=UTF-8''<title>`), para que el cliente Clash nombre el perfil importado con ese nombre.

El formato de los enlaces y el YAML generados se mantiene actualizado para los clientes modernos: Shadowsocks-2022 (SS2022) ya no codifica la información de usuario en Base64; los enlaces de Shadowsocks con ofuscación http se generan en formato SIP002 con el plugin `obfs-local`; para las suscripciones Clash/Mihomo se implementa el conjunto completo de campos XHTTP. No se requieren configuraciones separadas — los enlaces simplemente son reconocidos correctamente por los clientes.

> Nota: esta versión admite exactamente tres formatos — enlaces normales (Base64/texto), JSON (compatible con sing-box) y Clash/Mihomo (YAML). No hay un formato separado de Outline en el servidor de suscripciones.

### 10.4. Página de información de la suscripción y códigos QR

Si se abre el enlace de suscripción en un navegador (o se añade explícitamente el parámetro `?html=1` o `?view=html` a la URL, o se envía el encabezado `Accept: text/html`), el servidor devuelve una **página de información de la suscripción** visual («Información de suscripción») en lugar de la respuesta «raw». Las aplicaciones VPN siguen recibiendo la respuesta en formato de máquina, ya que no solicitan HTML.

La página (aplicación de una sola página compilada con Vite) muestra:

- **Información de la suscripción** (bloque Descriptions):
  - «ID de suscripción» — valor de `subId`;
  - «Estado» — «Activa», «Inactiva» o «Ilimitada». El estado «inactiva» se establece si el cliente está desactivado, ha agotado el límite de tráfico o ha vencido;
  - «Descargado» y «Enviado» — volúmenes de tráfico;
  - «Límite total» — límite de tráfico o `∞` si no tiene restricción;
  - «Fecha de vencimiento» — fecha de finalización o «Permanente»;
  - tráfico restante y hora del último acceso en línea.
  - Las fechas se muestran según el calendario gregoriano o Jalali en función de la configuración «Calendar Type» del panel (`datepicker`, por defecto `gregorian`).
- **Enlaces de suscripción**: para cada formato activado — una línea separada con una etiqueta de color (verde **SUB**, violeta **JSON**, dorado **CLASH**), botón de copia y botón de **código QR** (ventana emergente, tamaño 240 px). La línea con JSON y CLASH aparece solo si el formato correspondiente está activado en la configuración.
- **Enlaces individuales** («Copiar enlace»): lista completa de configuraciones individuales incluidas en la suscripción, cada una con su etiqueta de protocolo, botón de copia y código QR (para los enlaces post-quantum no se genera QR).

- **Botón «Copiar todas las configuraciones»** (sobre la lista de enlaces individuales): con un solo clic copia al portapapeles todos los enlaces de configuración (cada uno en una nueva línea), sin necesidad de copiarlos uno a uno; al finalizar se muestra la notificación «Todas las configuraciones copiadas».
- **Botones de importación rápida en aplicaciones** (menús desplegables por plataforma): para Android — v2box, v2rayNG (deep-link `v2rayng://install-config?url=…`), Sing-box, V2RayTun, NPV Tunnel, Happ (`happ://add/…`), Incy (`incy://add/…`); para iOS — Shadowrocket (mediante el parámetro `flag=shadowrocket`), v2box (`v2box://install-sub?url=…&name=…`), Streisand (`streisand://import/…`), V2RayTun, NPV Tunnel, Happ, Incy. Estos botones abren el deep-link de la aplicación correspondiente con la dirección de suscripción ya incorporada, o copian el enlace al portapapeles.

La página de información se devuelve con encabezados de no-caché (`Cache-Control: no-cache`), para que el cliente siempre vea los datos actualizados de tráfico y fecha de vencimiento.

### 10.5. Plantillas personalizadas de la página de suscripción

A partir de la versión 3.3.0 es posible reemplazar la página de inicio estándar de la suscripción con una plantilla HTML propia. Por defecto, en la dirección de suscripción se muestra la página integrada, pero si se especifica un directorio con una plantilla personalizada, el panel la renderizará e insertará en ella los datos actualizados del cliente (tráfico, fecha de vencimiento, enlaces, etc.).

Importante: el panel **no incluye** plantillas listas para usar — el tema propio hay que crearlo desde cero. Las instrucciones de creación y la lista de variables disponibles están en [`docs/custom-subscription-templates.md`](https://github.com/MHSanaei/3x-ui/blob/main/docs/custom-subscription-templates.md).

#### Dónde se activa

El directorio del tema se establece en la configuración del panel:

**Configuración → Suscripción → sección «Información de suscripción»**, campo **«Directorio del tema de suscripción»** (`subThemeDir`).

Descripción del campo en la interfaz:
«Ruta absoluta a la carpeta con la plantilla personalizada (index.html/sub.html) para la página de suscripción (por ejemplo, /etc/3x-ui/sub_templates/my-theme/). Déjelo vacío para usar la página predeterminada.»

En la misma sección, junto a este campo, se encuentran configuraciones relacionadas cuyos valores están disponibles en la plantilla:

En la descripción del campo «Directorio del tema de suscripción» hay un enlace **«Guía de plantillas ↗»** a la documentación para crear plantillas de diseño personalizadas de la página de suscripción.
- **«Título de suscripción»** (`subTitle`) — nombre visible para el cliente;
- **«URL de soporte»** (`subSupportUrl`) — enlace de soporte técnico.

#### Parámetro de configuración

| Parámetro | Valor predeterminado | Uso |
|---|---|---|
| `subThemeDir` | `""` (vacío) | Ruta absoluta al directorio con su plantilla HTML. Vacío = página integrada predeterminada. |

#### Cómo usar su propia plantilla

1. Cree en el servidor una carpeta para el tema (en cualquier lugar), por ejemplo `/etc/3x-ui/sub_templates/my-theme/`.
2. Coloque dentro un archivo HTML con el nombre `index.html` o `sub.html`.

**Ejemplo: ruta al tema.** La estructura final en el servidor y el valor del campo en la configuración:

```
/etc/3x-ui/sub_templates/my-theme/
└── index.html        (o sub.html — tiene prioridad)
```

```
Configuración → Suscripción → «Directorio del tema de suscripción»:
/etc/3x-ui/sub_templates/my-theme/
```

La ruta debe ser **absoluta** (comenzar con `/`). Si la carpeta no contiene ni `index.html` ni `sub.html`, el panel mostrará la página integrada.
3. En el panel abra **Configuración → Suscripción** e ingrese la ruta **absoluta** a esa carpeta en el campo «Directorio del tema de suscripción».
4. Guarde la configuración.

Comportamiento de selección de archivo y renderizado:
- Si en el directorio existe `sub.html`, se usa ese; de lo contrario se toma `index.html`. Es decir, `sub.html` tiene prioridad sobre `index.html`.
- La plantilla se renderiza con el motor estándar de Go `html/template`.
- La plantilla analizada se **almacena en caché** y se vuelve a leer del disco solo cuando cambia la hora de modificación del archivo. Por lo tanto, los cambios en la plantilla se capturan sin reiniciar el panel, pero sin la sobrecarga de lectura/análisis en cada solicitud.
- La respuesta se forma en un búfer completo y solo entonces se envía al cliente: si la plantilla falla durante la ejecución, la página parcialmente generada (rota) no llegará al usuario.

#### Comportamiento predeterminado y alternativa (fallback)

- Campo vacío → se muestra la página SPA integrada (los datos se inyectan en `window.__SUB_PAGE_DATA__`).
- La ruta no existe o no es un directorio → se usa la página predeterminada.
- El directorio no contiene ni `index.html` ni `sub.html` → se escribe en el log la advertencia «subThemeDir set but no usable template found», se muestra la página predeterminada.
- El archivo de plantilla existe pero no se puede analizar → se escribe en el log el error «custom template parse failed», se muestra la página predeterminada.
- Error al ejecutar la plantilla → se escribe en el log «custom template execution failed», se muestra la página predeterminada.

Es decir, cualquier problema con la plantilla personalizada no «rompe» la suscripción — el panel siempre cae a la página integrada. Todas las páginas de suscripción (tanto la personalizada como la estándar) se devuelven con encabezados de no-caché (`Cache-Control: no-cache, no-store, must-revalidate`), para que los clientes siempre reciban datos actualizados de tráfico y fecha de vencimiento.

#### Variables de plantilla disponibles

Al contexto de la plantilla se le pasa un conjunto de datos del cliente de la suscripción. El acceso es mediante `{{ .nombre }}`:

| Variable | Tipo | Descripción |
|---|---|---|
| `{{ .sId }}` | string | ID de suscripción (UUID). |
| `{{ .enabled }}` | bool | Si el cliente/suscripción está activado. |
| `{{ .download }}` | string | Volumen de descarga formateado (p. ej. «2.5 GB»). |
| `{{ .upload }}` | string | Volumen de envío formateado. |
| `{{ .total }}` | string | Límite de tráfico total formateado. |
| `{{ .used }}` | string | Tráfico utilizado formateado (download + upload). |
| `{{ .remained }}` | string | Tráfico restante formateado. |
| `{{ .expire }}` | int64 | Fecha de vencimiento — hora Unix en **segundos** (`0` = permanente). Para JS `Date` multiplique por 1000. |
| `{{ .lastOnline }}` | int64 | Hora del último acceso en línea — hora Unix en **milisegundos** (`0` = nunca). |
| `{{ .downloadByte }}` | int64 | Descarga en bytes exactos. |
| `{{ .uploadByte }}` | int64 | Envío en bytes exactos. |
| `{{ .totalByte }}` | int64 | Límite total en bytes exactos. |
| `{{ .subUrl }}` | string | URL de la página de suscripción. |
| `{{ .subJsonUrl }}` | string | URL de la configuración JSON de la suscripción. |
| `{{ .subClashUrl }}` | string | URL de la configuración Clash/Mihomo. |
| `{{ .subTitle }}` | string | Título de la suscripción desde la configuración (puede estar vacío). |
| `{{ .subSupportUrl }}` | string | URL de soporte desde la configuración (puede estar vacío). |
| `{{ .links }}` | []string | Lista de cadenas de configuración (VMess, VLESS, etc.). Iteración: `{{ range .links }} … {{ end }}`. |
| `{{ .emails }}` | []string | Lista de emails relacionados con la suscripción. |
| `{{ .datepicker }}` | string | Formato de calendario actual del panel: `gregorian` o `jalali` (tomado de la configuración «Tipo de calendario»; si está vacío — `gregorian`). |

Ejemplo mínimo del cuerpo de plantilla que usa algunas variables:

```html
<h1>{{ .subTitle }}</h1>
<p>Utilizado: {{ .used }} de {{ .total }} (restante {{ .remained }})</p>
{{ range .links }}<div>{{ . }}</div>{{ end }}
```

**Ejemplo: fecha de vencimiento desde `expire`.** El campo `{{ .expire }}` es la hora Unix en **segundos**, por lo que para JavaScript se multiplica por 1000; el valor `0` significa «sin vencimiento»:

```html
<script>
  var exp = {{ .expire }};
  document.write(exp === 0
    ? 'Sin vencimiento'
    : 'Hasta ' + new Date(exp * 1000).toLocaleDateString());
</script>
```

Tenga en cuenta que `{{ .lastOnline }}` ya está en **milisegundos** — no es necesario multiplicarlo por 1000.

---

## 11. Xray: enrutamiento, outbounds, DNS y extensiones

La sección **«Configuración de Xray»** es un editor de plantilla de configuración de Xray-core, a partir de la cual el panel genera el `config.json` final para ejecutar el núcleo. La pista de la sección de plantilla: *«A partir de la plantilla se crea el archivo de configuración de Xray.»* A diferencia de los inbounds (que se almacenan por separado en la BD y se insertan en la plantilla durante el ensamblado de la configuración), todo lo demás — logs, enrutamiento, outbounds, DNS, política, estadísticas — se define aquí.

> Importante: el valor de la plantilla se almacena en la BD bajo la clave `xrayTemplateConfig`. Al guardar, el panel lo procesa mediante una serie de transformaciones automáticas (véase [11.11](#1111-guardado-reinicio-y-transformaciones-automáticas)). Cualquier JSON sintácticamente incorrecto será rechazado con el error *«xray template config invalid»*.

#### Ubicación en el menú: «Salientes» y «Enrutamiento»

**«Salientes» (Outbounds)** y **«Enrutamiento» (Routing)** son elementos separados del menú lateral (justo debajo de «Hosts», sobre «Configuración del panel»), cada uno con su propia dirección: `/outbound` y `/routing`. Los enlaces directos a estas páginas y la recarga de página funcionan como se espera. En el submenú **«Configuraciones de Xray»** quedan únicamente: Principal, Balanceador, DNS y Plantilla avanzada. En la descripción a continuación, las secciones [11.3](#113-reglas-de-enrutamiento-routing) y [11.4](#114-outbounds-conexiones-salientes) corresponden a las páginas «Enrutamiento» y «Salientes».

### 11.1. Estructura del editor: pestañas/modos

El editor ofrece varios modos de visualización de la plantilla (filtros por secciones JSON):

| Modo | Qué muestra |
|---|---|
| **Principal** | Secciones básicas (Log, enrutamiento básico, configuración principal) |
| **Plantilla avanzada** | Plantilla JSON completa de Xray |
| **Todo** | Todas las secciones simultáneamente |

Grupos lógicos de configuración dentro del editor:

- **Configuración principal** (pista: *«Estos parámetros describen la configuración general»*).
- **Log** (véase [11.10](#1110-logs-y-estadísticas-stats-metrics)).
- **Conexiones básicas**: bloqueos y rutas directas.
- **Entrantes** (pista: *«Modificar la plantilla de configuración para conectar determinados clientes»*).
- **Salientes** (véase [11.4](#114-outbounds-conexiones-salientes)).
- **Balanceador** (véase [11.5](#115-balanceadores-balancers)).
- **Enrutamiento** (pista: *«¡La prioridad de cada regla es importante!»*, véase [11.3](#113-reglas-de-enrutamiento-routing)).
- **DNS / Fake DNS** (véase [11.6](#116-dns)).

### 11.2. Configuración principal (General)

#### Freedom Protocol Strategy

| Campo | Etiqueta | Descripción | Por defecto |
|---|---|---|---|
| `FreedomStrategy` | **Configuración de la estrategia del protocolo Freedom** | Estrategia de salida de red para el outbound directo (freedom). Pista: *«Configurar la estrategia de salida de red en el protocolo Freedom»*. Controla el campo `domainStrategy` dentro de `settings` del outbound con protocolo `freedom`. | En la plantilla de referencia, `domainStrategy` para el freedom-outbound `direct` es **`AsIs`** (la dirección no se resuelve, se transmite tal cual). |

`domainStrategy` para freedom (valores de Xray-core): `AsIs` (no resolver el dominio en el lado del servidor), así como la familia `UseIP` / `UseIPv4` / `UseIPv6` y sus variantes «forzadas» `ForceIP*`, que obligan al servidor de salida a resolver el dominio y conectarse por la IP obtenida. Cambie a `UseIPv4` si el servidor de salida no tiene IPv6 o si necesita forzar el uso exclusivo de IPv4.

#### Freedom Happy Eyeballs (IPv4/IPv6)

| Campo | Etiqueta | Descripción |
|---|---|---|
| `FreedomHappyEyeballs` | **Freedom Happy Eyeballs (IPv4/IPv6)** | Pista: *«Marcación dual para la salida directa (freedom) — útil en servidores de salida con IPv4 e IPv6.»* Activa el algoritmo Happy Eyeballs (intento simultáneo con ambas familias de direcciones) para el freedom-outbound. |
| try delay | (pista) | *«Milisegundos antes de intentar con otra familia de direcciones. 150–250 ms es un buen punto de partida.»* Retardo antes de cambiar a la familia de direcciones alternativa. El rango recomendado es 150–250 ms. |

#### Overall Routing Strategy

| Campo | Etiqueta | Descripción | Por defecto |
|---|---|---|---|
| `RoutingStrategy` | **Configuración de enrutamiento de dominios** | Estrategia general de resolución DNS para el enrutamiento. Pista: *«Configurar la estrategia general de enrutamiento de resolución DNS»*. Controla el campo `routing.domainStrategy`. | En la plantilla de referencia, `routing.domainStrategy` = **`AsIs`**. |

`routing.domainStrategy` determina cómo se comparan las reglas de enrutamiento de IP con las solicitudes de dominio: `AsIs` (solo reglas de dominio, sin resolución), `IPIfNonMatch` (si el dominio no coincide con las reglas, resolver y comprobar reglas de IP), `IPOnDemand` (resolver inmediatamente al encontrar una regla de IP). Para que las reglas de IP (por ejemplo, `geoip:*`) funcionen con solicitudes de dominio, generalmente se requiere `IPIfNonMatch`.

#### Outbound Test URL

| Campo | Etiqueta | Descripción | Por defecto |
|---|---|---|---|
| `outboundTestUrl` | **URL para prueba de saliente** | URL para verificar la conectividad al probar el outbound. Pista: *«URL para comprobar la conectividad del saliente»*. Se almacena por separado de la plantilla, bajo la clave `xrayOutboundTestUrl`. | **`https://www.google.com/generate_204`** |

El valor pasa por saneamiento. Durante la prueba del outbound se verifica adicionalmente como URL público — esto es una protección contra SSRF: el usuario no puede inyectar una URL arbitraria (incluidas las internas) a través del cliente; la URL de prueba siempre se toma de la configuración del servidor. Un valor vacío al guardar/probar se reemplaza por el `generate_204` predeterminado.

#### Block BitTorrent

| Campo | Etiqueta | Descripción |
|---|---|---|
| `Torrent` | **Bloquear BitTorrent** | Agrega a `routing.rules` una regla que envía el tráfico con `protocol: ["bittorrent"]` al outbound `blocked`. En la plantilla de referencia esta regla está presente por defecto. |

#### Límites de conexión (Connection Limits)

Pista: *«Políticas de nivel de conexión para usuarios de nivel 0. Deje el campo vacío para usar el valor predeterminado de Xray.»* Estos parámetros se escriben en `policy.levels.0`.

| Campo | Etiqueta | Descripción | Por defecto |
|---|---|---|---|
| `connIdle` | **Tiempo de espera de inactividad** (segundos) | *«Cierra la conexión tras el número de segundos de inactividad indicado. Reducir el valor libera memoria y descriptores de archivo más rápido en servidores con alta carga (valor predeterminado en Xray: 300).»* | vacío → predeterminado Xray **300** |
| `bufferSize` | **Tamaño de búfer** (KB) | *«Tamaño del búfer interno por conexión en KB. Establezca 0 para minimizar el uso de memoria en servidores con poca RAM (el valor predeterminado de Xray depende de la plataforma).»* Marcador de posición: **«auto»**. | vacío → depende de la plataforma; `0` — minimizar |

**Ejemplo (`policy.levels.0`).** Los campos de este grupo se escriben en la política de nivel 0. En un servidor con alta carga y poca RAM se pueden liberar recursos más rápido así:

```json
"policy": {
  "levels": {
    "0": {
      "connIdle": 120,
      "bufferSize": 0
    }
  }
}
```

Aquí la conexión se cierra tras 120 s de inactividad (en lugar del predeterminado 300), y `bufferSize: 0` minimiza el consumo de memoria en búferes. Un campo dejado vacío en el formulario simplemente no se incluirá en el JSON, y Xray aplicará su valor predeterminado.

### 11.3. Reglas de enrutamiento (routing)

Lista de reglas `routing.rules`. **El orden es crítico** (*«¡La prioridad de cada regla es importante!»*): las reglas se evalúan de arriba abajo, se aplica la primera coincidencia. Pista: *«Arrastre para cambiar el orden»*. Botones de control de orden: **Primero**, **Último**, **Subir**, **Bajar**.

Cada regla tiene `type: "field"`. Botones: **Crear regla**, **Editar regla**. Pista para campos de lista: *«Elementos separados por comas»*.

En la página «Enrutamiento», los botones **«Importar reglas»** y **«Exportar reglas»** están agrupados en el menú desplegable **«más»** (more), igual que en la página «Salientes». El botón **«Exportar reglas»** no descarga el archivo inmediatamente, sino que abre una ventana modal con vista previa del JSON y los botones **«Copiar»** y **«Descargar»**: el contenido puede revisarse antes de guardar. La exportación de salientes en la página «Salientes» funciona de manera análoga.

#### Route Tester (probador de ruta)

En la pestaña Routing hay una subpestaña **Route Tester** — pregunta al Xray en ejecución qué outbound procesaría una conexión específica, sin enviar tráfico real. Especifique un dominio o IP, puerto, red (TCP/UDP) y, si es necesario, inbound y protocolo interceptado (`http`/`tls`/`quic`/`bittorrent`), luego haga clic en **Test Route**. La decisión se obtiene directamente del motor de enrutamiento en vivo.

En la respuesta se muestra el outbound seleccionado y, al usar un balanceador, también la etiqueta del balanceador. Si ninguna regla coincide, el probador informa que el tráfico va al outbound predeterminado (el primero en la lista `outbounds`). Esto resulta útil para verificar el orden de las reglas antes de confiar en ellas.

#### Activar y desactivar una regla individual

Una regla de enrutamiento individual puede **desactivarse** temporalmente con un interruptor, sin eliminarla. En la tabla de reglas hay una columna **«Activar»** con un interruptor (Switch), y en el formulario de la regla hay un campo **«Activar»** que también es un interruptor. Una regla desactivada no se incluye en la configuración final de Xray, pero se conserva en la plantilla y puede reactivarse en cualquier momento.

La regla de servicio de estadísticas (`inboundTag: ["api"] → outboundTag: "api"`) no se puede desactivar — su interruptor está bloqueado para no romper la contabilidad de tráfico del panel (véase [11.11](#1111-guardado-reinicio-y-transformaciones-automáticas)).

#### Campos del formulario de regla

| Campo del formulario | Etiqueta | Campo JSON | Descripción |
|---|---|---|---|
| Origen | **Origen** | `source` | Direcciones IP/subredes de origen. Lista separada por comas. |
| Puerto de origen | **Puerto de origen** | `sourcePort` | Puerto(s) de origen. |
| Destino | **Destino** | `domain` + `ip` + `port` | Dominios, IP y puertos de destino. Los dominios admiten prefijos `domain:`, `full:`, `regexp:`, `keyword:`, así como `geosite:*`; las IP admiten `geoip:*` y CIDR. |
| Red | — | `network` | `tcp`, `udp` o `tcp,udp`. |
| Protocolo | — | `protocol` | `http`, `tls`, `bittorrent` (determinado por sniffing). |
| Usuario | **Usuario** | `user` | Filtro por correo electrónico/identificador de usuario. |
| Atributos / Valor | **Atributos** / **Valor** | `attrs` | Atributos de encabezados HTTP para comparación. |
| VLESS route | **VLESS route** | — | Enrutamiento por campo route para VLESS. |
| Etiquetas de entrantes | **Etiquetas de entrantes** | `inboundTag` | Una o más etiquetas de inbound a las que se aplica la regla (incluidas las integradas `api` y la etiqueta DNS de la configuración DNS). En las listas de inbound se muestra como «etiqueta (nota)» si el inbound tiene una nota separada, de lo contrario solo la etiqueta; en las reglas guardadas solo se almacenan las etiquetas. |
| Etiqueta del saliente | **Etiqueta del saliente** / **Conexión saliente** | `outboundTag` | A dónde dirigir el tráfico coincidente. |
| Etiqueta del balanceador | **Etiqueta del balanceador** / **Balanceador** | `balancerTag` | Pista: *«Dirige el tráfico a través de uno de los balanceadores de carga configurados»*. |

> Exclusión mutua de `outboundTag` y `balancerTag`: *«No es posible usar balancerTag y outboundTag al mismo tiempo. Si se usan simultáneamente, solo funcionará outboundTag.»* En una regla, especifique solo la etiqueta del saliente o la etiqueta del balanceador.

#### Reglas integradas de la plantilla de referencia

En el `config.json` estándar, la sección `routing` contiene tres reglas (en este orden):

1. `inboundTag: ["api"] → outboundTag: "api"` — regla de servicio para la API gRPC de estadísticas del panel.
2. `ip: ["geoip:private"] → outboundTag: "blocked"` — bloqueo de rangos privados.
3. `protocol: ["bittorrent"] → outboundTag: "blocked"` — bloqueo de BitTorrent.

> La regla `api → api` siempre se eleva automáticamente a la posición 0 al guardar (véase [11.11](#1111-guardado-reinicio-y-transformaciones-automáticas)), para que una regla catch-all superior no «consuma» la solicitud de estadísticas.

**Ejemplo de regla.** Enviar todo el tráfico a sitios rusos y redes privadas directamente (sin proxy), y el resto al balanceador. El orden importa: la regla «dirigir directamente» debe estar por encima del catch-all. En `routing.rules`:

```json
{
  "type": "field",
  "domain": ["geosite:category-ru", "domain:example.ru"],
  "ip": ["geoip:ru", "geoip:private"],
  "outboundTag": "direct"
}
```

Para que las reglas de IP (`geoip:ru`) también se apliquen a las solicitudes de dominio, generalmente se necesita `routing.domainStrategy: "IPIfNonMatch"` en el nivel superior del enrutamiento (véase [11.2](#112-configuración-principal-general)).

#### Grupos de enrutamiento preconfigurados (Conexiones básicas)

En el modo «Conexiones básicas», el panel ayuda a ensamblar reglas típicas a partir de listas predefinidas:

| Grupo | Campos | Pista |
|---|---|---|
| Bloqueo por protocolos/sitios | — | *«Configure para que los clientes no tengan acceso a determinados protocolos»* |
| Bloqueo por países | **Direcciones IP bloqueadas**, **Dominios bloqueados** | *«Estos parámetros bloquearán el tráfico según el país de destino.»* |
| Conexiones directas | **IPs directas**, **Dominios directos** | *«La conexión directa significa que cierto tráfico no se redirigirá a través de otro servidor.»* |
| Reglas IPv4 | — | *«Estos parámetros permitirán a los clientes enrutar hacia dominios de destino solo a través de IPv4»* |
| Reglas WARP | — | *«Estas opciones dirigirán el tráfico según el destino específico a través de WARP.»* |
| Enrutamiento NordVPN | — | *«Estas opciones dirigirán el tráfico según el destino específico a través de NordVPN.»* |

#### MTProto-inbound: enrutamiento del tráfico de Telegram a través de Xray

El MTProto-inbound tiene un interruptor **«Route through Xray»** (desactivado por defecto) y una selección opcional de **Outbound**. Al activarlo, el panel agrega al config de Xray un puente SOCKS de loopback con la etiqueta del propio inbound, y mtg dirige el tráfico de Telegram a través de él. A partir de entonces, el enrutador controla el tráfico saliente de Telegram: puede compararse con reglas normales en la pestaña Routing por la etiqueta del inbound, o forzarse a un outbound o balanceador seleccionado mediante el campo **Outbound**. Deje **Outbound** vacío para que las reglas de enrutamiento tomen la decisión.

### 11.4. Outbounds (conexiones salientes)

Lista de `outbounds`. Botones: **Crear conexión saliente**, **Editar conexión saliente**. Pista: *«Modificar la plantilla de configuración para definir las conexiones salientes de este servidor»*.

En la plantilla de referencia hay dos outbounds obligatorios:

- `protocol: "freedom"`, `tag: "direct"` — salida directa a internet (con `domainStrategy: "AsIs"` y `finalRules: [{action: "allow"}]`);
- `protocol: "blackhole"`, `tag: "blocked"` — «agujero negro» para el tráfico bloqueado.

#### Campos generales del formulario de outbound

| Campo | Etiqueta | Descripción |
|---|---|---|
| Etiqueta | **Etiqueta** (pista: *«Etiqueta única»*) | Identificador único del outbound. Marcador de posición: *«etiqueta-única»*. Validación: *«La etiqueta es obligatoria»*, *«La etiqueta ya está en uso por otro saliente»*. |
| Protocolo | — | Tipo de saliente (véase más abajo). |
| Dirección / Puerto | **Dirección** / Puerto | Destino de la conexión. La dirección y el puerto son obligatorios. |
| Enviar a través de | **Enviar a través de** | Dirección IP local de la interfaz saliente (`sendThrough`). Marcador de posición: *«IP local»*. |
| Dialer proxy (cadena) | — | Pista: *«Conecte este saliente a través de otro saliente (por etiqueta) para construir una cadena de proxies. Deje vacío para conexión directa.»* Marcador de posición: *«Seleccione un saliente para encadenar»*. Se implementa mediante `streamSettings.sockopt.dialerProxy`. |

La lista desplegable **Dialer Proxy** muestra no solo los outbounds locales, sino también las etiquetas de outbounds de suscripciones — así se puede construir una cadena también a través de una salida obtenida por suscripción. De la lista siguen excluidos el outbound blackhole y el propio outbound en edición. Deje el campo vacío para conexión directa.

#### Protocolos de outbound soportados

Protocolos soportados por el formulario:

- **`freedom`** — salida directa. Campos `settings.domainStrategy`, `finalRules` (véase más abajo), Happy Eyeballs. No se puede probar (*«Outbound has no testable endpoint»*).
- **`blackhole`** — descarta el tráfico. Campo **Tipo de respuesta**. No se puede probar.
- **`socks`**, **`http`** — lista `settings.servers[]` con `address`/`port`; campo **Contraseña de autorización**. Para el protocolo **`http`**, debajo de los campos **Username**/**Password** hay un editor **Headers** (Encabezados) — pares clave/valor para los encabezados CONNECT enviados al proxy HTTP ascendente. Estos encabezados se conservan al volver a abrir y guardar el outbound (antes se perdían). Tenga en cuenta: solo se aplican los encabezados a nivel de configuración (`settings.headers`); los encabezados a nivel de servidor individual son ignorados por xray-core.
- **`vmess`** — `settings.vnext[]` (`address`/`port`).
- **`vless`** — `settings.address`/`settings.port`.
- **`trojan`**, **`shadowsocks`** — `settings.servers[]`.
- **`wireguard`** — `settings.peers[]` con `endpoint`, más claves (véase [11.8](#118-wireguard--warp--nordvpn)).
- **`hysteria`** — `settings.address`/`settings.port` (transporte UDP).

Para el outbound de tipo **loopback** está disponible el bloque **Sniffing** con los mismos parámetros que en el inbound: activación, **destOverride**, **Metadata Only**, **Route Only** y la lista de **dominios excluidos**.

En la máscara **UDP** (FinalMask) para **Hysteria2** hay modos adicionales disponibles. La máscara **Salamander** tiene un selector **Mode** con los valores **Salamander** y **Gecko**: el modo Gecko agrega relleno aleatorio de paquetes con los campos **Min**/**Max** de tamaño (`packetSize`, rango 1–2048, por defecto 512–1200) — esto protege contra la toma de huella digital por longitud de paquete. La máscara **Realm** (UDP hole-punching) tiene un bloque opcional **TLS Config** con los campos **Server Name** (SNI), **ALPN** (`h3`/`h2`/`http/1.1`), **Fingerprint** (uTLS) y el interruptor **Allow Insecure**.

**Ejemplo: cadena a través de un SOCKS ascendente.** El outbound `upstream` conecta a un proxy SOCKS5 externo, y `chained` envía su tráfico a través de él (`dialerProxy`), formando una cadena. En `outbounds`:

```json
[
  {
    "tag": "upstream",
    "protocol": "socks",
    "settings": {
      "servers": [{ "address": "203.0.113.10", "port": 1080 }]
    }
  },
  {
    "tag": "chained",
    "protocol": "freedom",
    "streamSettings": {
      "sockopt": { "dialerProxy": "upstream" }
    }
  }
]
```

Ahora una regla de enrutamiento con `outboundTag: "chained"` enviará el tráfico a internet a través de `upstream`.

#### Importar outbound desde enlace compartido

Un outbound se puede importar desde un enlace compartido (`vless://`, `vmess://`, etc.). Al importar también se conservan la configuración del multiplexor **xmux** (XHTTP) transmitida en el bloque `extra=` del enlace: tras la importación, sus valores se insertan en el subformulario **XMUX** del outbound creado.

#### Campos Mux (multiplexación)

**Máx. paralelismo**, **Máx. conexiones**, **Máx. reutilizaciones**, **Máx. solicitudes**, **Máx. segundos de reutilización**, **Período keep alive**. Estos parámetros configuran el comportamiento mux/XUDP del saliente.

#### Sockopts (configuración de socket)

Grupo **Sockopts**: **Intervalo keep alive**, **Mark (fwmark)**, **Interfaz**, **Solo IPv6**, **Aceptar proxy protocol**, **Proxy protocol**, **TCP user timeout (ms)**, **TCP keep-alive idle (s)**. Aquí también se configura el dialer-proxy de la cadena.

#### Freedom finalRules (anular el bloqueo de IPs privadas)

Para el freedom-outbound está disponible el grupo **Reglas finales**:

| Campo | Etiqueta | Descripción |
|---|---|---|
| `overrideXrayPrivateIp` | **Anular el bloqueo predeterminado de IPs privadas en Xray** | Elimina la prohibición integrada de Xray para salientes hacia IPs privadas. |
| `action` | **Acción** | `allow` (como en la plantilla de referencia: `finalRules: [{action: "allow"}]`), `redirect` (**Redirect**) u otras. |
| `blockDelay` | **Retraso de bloqueo (ms)** | Retraso antes de descartar la conexión. |
| `redirect` / `fragment` | **Redirect** / **Fragment** | Acciones de redirección y fragmentación del tráfico. |

#### Máscara fragment: Lengths y Delays por fragmento

En la máscara **fragment** (tipo fragment en FinalMask, para TCP), los campos únicos Length y Delay se reemplazan por las listas **Lengths** y **Delays**: para cada segmento se puede especificar un rango de longitud separado (por ejemplo `100-200`) y retrasos en milisegundos (por ejemplo `10-20` o `0`). Las filas de las listas se pueden agregar y eliminar; los valores únicos guardados anteriormente se transfieren automáticamente a un arreglo de un elemento.

#### Otros campos del formulario

- **UDP over TCP** y **Versión UoT** — para protocolos similares a shadowsocks.
- **Sin encabezado gRPC**, **Tamaño de chunk Uplink** — parámetros de transporte gRPC.
- Campos TLS/uTLS: **Verificar nombre del peer**, **Pinned SHA256**, **Short ID**, **Vision testpre**, marcador de posición «nombre del servidor».

#### Prueba de salientes

Botones: **Probar**, **Probar todos**. Estados: **Probando conexión...**, **Prueba exitosa**, **Prueba fallida**, **No se pudo probar la conexión saliente**. Resultado: **Resultado de la prueba**, latencia en milisegundos.

Dos modos (pista: *«TCP: sondeo rápido solo de dial. HTTP: solicitud completa a través de xray.»*):

- **TCP** (`mode=tcp`) — dial simple hasta `host:port`, se ejecuta en paralelo para todos los endpoints, ~tiempo de espera 5 s. Verifica solo la accesibilidad TCP, no valida el protocolo proxy. Para `freedom`/`blackhole`/etiqueta `blocked` devolverá *«Outbound has no testable endpoint»*.
- **HTTP** (`mode=http` o vacío) — levanta una instancia temporal de Xray, ejecuta una solicitud HTTP real (URL de sondeo = `outboundTestUrl` del servidor), mide la latencia real. Es el modo autoritativo, pero costoso: se serializa con un bloqueo global (*«Another outbound test is already running, please wait»*). El tiempo de espera de un intento es de 10 s, el margen de espera del resultado es de 15 s (aumentados para que outbounds sanos en canales lentos o tunelizados no se marquen como «Failed»). En caso de fallo, la causa real (error DNS, connection refused, expiración del deadline, error TLS, etc.) se escribe en el log del panel/Xray, al que apuntan los mensajes generales de tiempo de espera.

> Los protocolos UDP (`wireguard`, `hysteria`) y los transportes UDP (`kcp`, `quic`, `hysteria`) **siempre** se prueban en modo HTTP, incluso si se solicita TCP — un UDP-dial simple no distingue un endpoint «vivo» de uno «muerto». Para wireguard en la configuración de prueba se fuerza `noKernelTun: true`.

#### Verificación por lotes y desglose por etapas

**Probar** y **Probar todos** en modo HTTP levantan una instancia temporal común de Xray para el lote de outbounds, crean un inbound SOCKS de loopback con regla para cada uno y envían en paralelo una solicitud HTTP real a través de él; **Probar todos** verifica los outbounds por lotes. **Probar todos** también verifica los outbounds obtenidos de suscripciones (tabla «de suscripciones», solo lectura) — sus filas también se resaltan con el resultado de la prueba. Los outbounds `freedom` («direct») y `dns` no se prueban en ningún modo (no son proxies): el botón de prueba no está disponible para ellos, **Probar todos** los omite, y la protección del servidor prohíbe su prueba HTTP incluso con llamada directa a la API. Además del éxito/error, el popup de resultado muestra el estado HTTP de la respuesta y el desglose del tiempo por etapas: **Proxy connect** (conexión al proxy), **TLS via outbound** (TLS a través del outbound) y **First byte** (tiempo hasta el primer byte) — esto ayuda a entender en qué paso se produjo la latencia o el fallo.

#### Estadísticas de tráfico de outbounds

El panel lleva contadores de tráfico por etiquetas (`up`/`down`/`total`). El botón de reinicio restablece los contadores para una etiqueta específica o para todas (`tag = "-alltags-"`). Los campos **Información de la cuenta** y **Estado de la conexión saliente** muestran un resumen.

### 11.5. Balanceadores (Balancers)

Lista de `routing.balancers`. Botones: **Crear balanceador**, **Editar balanceador**.

En la pestaña Balancers hay columnas de estado en vivo: **Live Target** muestra el destino activo actual del balanceador en el Xray en ejecución, y **Override** permite anular manualmente la selección del destino (el valor **Auto (strategy)** devuelve la selección según la estrategia). El estado se actualiza con un botón separado. Si el balanceador aún no está activo en el Xray en ejecución, el panel sugerirá guardar primero los cambios o iniciar Xray.

| Campo | Etiqueta | Descripción |
|---|---|---|
| Etiqueta | **Etiqueta** (pista: *«Etiqueta única»*) | Identificador único. Marcador de posición: *«etiqueta única del balanceador»*. Validación: *«La etiqueta es obligatoria»*, *«La etiqueta ya está en uso por otro balanceador»*. |
| Selectores | **Selectores** | Lista de etiquetas de outbound (por subcadena) entre las que el balanceador elige la salida. Se debe seleccionar al menos uno: *«Seleccione al menos un saliente»*. |
| Fallback | **Fallback** | Etiqueta de outbound de respaldo si ningún selector coincide. |
| Estrategia | **Estrategia** | Algoritmo de selección (véase más abajo). |

#### Estrategia y parámetros de observación

La estrategia (`strategy.type`) determina cómo el balanceador elige el outbound entre los selectores. Valores de Xray-core: `random` (aleatorio), `roundRobin` (por turnos), `leastPing` (latencia mínima según los resultados del observatory), `leastLoad` (carga mínima). Para `leastLoad`/`leastPing` se usan los parámetros de `strategy.settings`:

| Campo | Etiqueta | Descripción |
|---|---|---|
| `expected` | **Esperado** | Marcador de posición: *«número óptimo de nodos»* — número objetivo de nodos activos. |
| `maxRtt` | **RTT máx.** | Límite superior del RTT admisible al seleccionar candidatos. |
| `tolerance` | **Tolerancia** | Tolerancia al comparar latencias/cargas. |
| `baselines` | **Baselines** | Umbrales de latencia para agrupar nodos. |
| `costs` | **Costs** | Coeficientes de peso (cost) para etiquetas individuales. |

**Ejemplos de estrategias.** El bloque `strategy` vive dentro del balanceador (en JSON, junto a `tag` y `selector`):

```json
"strategy": { "type": "random" }      // selección aleatoria entre selectores
"strategy": { "type": "roundRobin" }  // por turnos, alternando
"strategy": { "type": "leastPing" }   // latencia mínima (requiere observador)
```

Para `leastLoad` los parámetros se especifican en `settings`:

```json
"strategy": {
  "type": "leastLoad",
  "settings": {
    "expected": 2,
    "maxRTT": "1s",
    "tolerance": 0.05,
    "baselines": ["500ms", "1s", "2s"],
    "costs": [
      { "regexp": false, "match": "proxy-premium",   "value": 0.1 },
      { "regexp": true,  "match": "^proxy-cheap-.+$", "value": 5 }
    ]
  }
}
```

**Cómo funciona (con ejemplo).** Supongamos que el observador midió latencias para las salidas: `A = 250 ms`, `B = 280 ms`, `C = 700 ms`, `D = 1500 ms`. Con la configuración anterior, la selección es:

1. **`maxRTT: "1s"`** — las salidas con latencia superior a 1 s se descartan: `D` (1500 ms) queda eliminado. Quedan `A`, `B`, `C`.
2. **`baselines` + `expected`** — las salidas se agrupan por umbrales de latencia, y se toma el **menor** umbral en el que caigan al menos `expected` salidas. El umbral `500ms` ya contiene `A` y `B` — son 2 (= `expected`), por lo que se selecciona el grupo {`A`, `B`}. `C` (700 ms) no entra en la selección mientras haya suficientes salidas rápidas (es la «reserva caliente»).
3. **`tolerance: 0.05`** — dentro del grupo seleccionado, las salidas cuyas latencias difieren en no más de un 5% se consideran equivalentes, y la carga se reparte entre ellas por igual. `A` (250) y `B` (280) difieren en ~12% (> 5%), por lo que en igualdad de condiciones se prefiere la más rápida `A`; si la diferencia estuviera dentro del 5%, el tráfico iría tanto por `A` como por `B`.
4. **`costs`** — antes de la comparación, ajustan el «coste» de las salidas individuales: un `value` menor hace la salida más atractiva, uno mayor lo contrario. En el ejemplo `proxy-premium` obtiene `0.1` (se vuelve «más barato» y se selecciona con más frecuencia), y todos los `proxy-cheap-*` (por expresión regular, `regexp: true`) obtienen `5` (se vuelven «más caros» y se usan en último lugar). Así se pueden priorizar suavemente las salidas sin excluirlas de forma estricta.

Resultado: el tráfico irá principalmente por `A` (si las latencias son similares, a partes iguales con `B`), `C` quedará como reserva y `D` estará excluido hasta que su RTT baje del `maxRTT`.

#### Observador: `observatory` y `burstObservatory` (mediciones para `leastPing` / `leastLoad`)

Las estrategias `leastPing` y `leastLoad` no miden nada por sí solas — necesitan datos de latencia y disponibilidad de cada outbound. Estos los recopila el **observador** (observatory): periódicamente «pingea» cada outbound monitoreado y registra el tiempo de respuesta y la disponibilidad. Los mismos datos se muestran en la pestaña **«Observatorio»** (estados **Activo / No disponible**, **«Última actividad»**, **«Último intento»**).

No hay un formulario separado para el observador en el panel — el bloque se agrega **manualmente** en el editor de configuración de Xray, en el nivel superior del config (junto a `routing` y `outbounds`), y luego hay que **reiniciar Xray**.

Hay dos variantes disponibles:

- **`observatory`** — simple: `subjectSelector` + `probeURL` + `probeInterval`.
- **`burstObservatory`** — avanzado, con configuración detallada del ping a través de `pingConfig`; conveniente para múltiples salidas.

Ejemplo de bloque `burstObservatory`:

```json
{
  "subjectSelector": ["WS-SE", "WS-FR", "WS-PL"],
  "pingConfig": {
    "destination": "https://www.google.com/generate_204",
    "interval": "1m",
    "connectivity": "http://connectivitycheck.platform.hicloud.com/generate_204",
    "timeout": "5s",
    "sampling": 2
  }
}
```

Descripción de los campos:

| Campo | Qué define |
|---|---|
| `subjectSelector` | Lista de **prefijos de etiquetas** de outbound para monitorear. Xray toma todos los outbounds cuyas etiquetas comiencen con las cadenas indicadas. En el ejemplo se monitorean las salidas `WS-SE…`, `WS-FR…`, `WS-PL…`. Estas etiquetas deben coincidir con las seleccionadas en los **Selectores** del balanceador. |
| `pingConfig.destination` | URL solicitada **a través de cada outbound** para medir la latencia. Se usa una «página ligera» con respuesta `204` sin cuerpo — por ejemplo `https://www.google.com/generate_204`. El tiempo hasta la respuesta es la latencia medida. |
| `pingConfig.interval` | Con qué frecuencia pingear cada outbound. Cadena de duración: `"1m"` — una vez por minuto, también `"30s"`, `"5m"`, etc. Con más frecuencia los datos son más frescos, pero hay más tráfico de fondo. |
| `pingConfig.connectivity` | (opcional) URL para verificar la **conectividad básica** del propio servidor. Si no es accesible — significa que el problema está en la red del servidor, y el observador **no** marca el outbound como no disponible (protección contra falsos positivos por fallo local). Normalmente también es un endpoint con respuesta `204`. |
| `pingConfig.timeout` | Cuánto esperar la respuesta a un ping antes de considerar el intento fallido (por ejemplo `"5s"`). |
| `pingConfig.sampling` | Cuántas mediciones recientes almacenar y promediar por outbound. `2` — considerar los dos últimos pings (suaviza los picos aleatorios). |

Cómo conectar todo:

1. En el editor de Xray, agregue el bloque `burstObservatory` con los `subjectSelector` necesarios.
2. Cree un balanceador: **Estrategia** = `leastPing`, en los **Selectores** especifique las mismas etiquetas de outbound (`WS-SE`, `WS-FR`, `WS-PL`).
3. Dirija el tráfico hacia él con una regla de enrutamiento (campo **Etiqueta del balanceador**, véase [11.3](#113-reglas-de-enrutamiento-routing)).
4. Reinicie Xray. En la pestaña **«Observatorio»** aparecerán los estados de las salidas, y el balanceador comenzará a seleccionar la más rápida de las activas.

> En una regla no se puede especificar simultáneamente `balancerTag` y `outboundTag` — solo funcionará `outboundTag`.

### 11.6. DNS

Sección `dns`. Activación: **Activar DNS** (pista: *«Activar el servidor DNS integrado»*).

#### Parámetros generales de DNS

| Campo | Etiqueta | JSON | Descripción / pista |
|---|---|---|---|
| `tag` | **Nombre de etiqueta DNS** | `dns.tag` | *«Esta etiqueta estará disponible como etiqueta de entrante en las reglas de enrutamiento.»* Permite enrutar las propias solicitudes DNS a través de `inboundTag`. |
| `clientIp` | **IP del cliente** | `dns.clientIp` | *«Se usa para notificar al servidor la ubicación IP indicada durante las solicitudes DNS»* (EDNS Client Subnet). |
| `strategy` | **Estrategia de consulta** | `dns.queryStrategy` | *«Estrategia general de resolución de nombres de dominio»*. Valores: `UseIP`, `UseIPv4`, `UseIPv6`. |
| `disableCache` | **Desactivar caché** | `dns.disableCache` | *«Desactiva el almacenamiento en caché de DNS»*. |
| `disableFallback` | **Desactivar DNS de respaldo** | `dns.disableFallback` | *«Desactiva las solicitudes DNS de respaldo»*. |
| `disableFallbackIfMatch` | **Desactivar DNS de respaldo si hay coincidencia** | `dns.disableFallbackIfMatch` | *«Desactiva las solicitudes DNS de respaldo al coincidir con la lista de dominios del servidor DNS»*. |
| `enableParallelQuery` | **Activar consultas paralelas** | — | *«Activar consultas DNS paralelas a múltiples servidores para una resolución más rápida»*. |
| `useSystemHosts` | **Usar Hosts del sistema** | `dns.useSystemHosts` | *«Usar el archivo hosts del sistema instalado»*. |

**Ejemplo de bloque `dns`.** Las solicitudes a dominios de Google se resuelven a través del servidor DoH de Cloudflare, todo lo demás a través de `1.1.1.1`; para las solicitudes de Google se esperan solo IPs no privadas. En el nivel superior del config:

```json
"dns": {
  "tag": "dns-inbound",
  "queryStrategy": "UseIPv4",
  "servers": [
    {
      "address": "https://cloudflare-dns.com/dns-query",
      "domains": ["geosite:google"],
      "expectIPs": ["geoip:!private"]
    },
    "1.1.1.1"
  ]
}
```

Una cadena de servidor (`"1.1.1.1"`) sin campos es el servidor predeterminado para todos los demás dominios. La etiqueta `dns-inbound` puede usarse luego como `inboundTag` en las reglas de enrutamiento para dirigir las propias solicitudes DNS a través del outbound correcto.

#### Caché de registros obsoletos

| Campo | Etiqueta | Descripción |
|---|---|---|
| `serveStale` | **Usar obsoletos** | *«Devolver resultados obsoletos de la caché mientras se actualiza en segundo plano»*. |
| `serveExpiredTTL` | **TTL de obsoletos** | *«Tiempo de vida (segundos) de los registros de caché obsoletos; 0 = sin límite»*. |

#### Servidores DNS (lista `dns.servers`)

Botones: **Crear DNS**, **Editar DNS**, **Eliminar todos** (confirmación: *«Todos los servidores DNS se eliminarán de la lista. Esta acción no se puede deshacer.»*). Plantillas: **Usar plantilla**, ventana **Plantillas DNS**, incluido el preset **Familiar**.

Al hacer clic en **Editar DNS** en un registro de servidor DNS (igual que en un registro de Fake DNS), la ventana de edición carga los valores guardados del servidor, no los valores predeterminados.

Campos del servidor DNS:

| Campo | Etiqueta | Descripción |
|---|---|---|
| address | — | Dirección DNS (IP, URL DoH, `localhost`, `fakedns`, etc.). |
| `domains` | **Dominios** | Lista de dominios para los que se usa este servidor. |
| `expectIPs` | **IPs esperadas** | Aceptar la respuesta solo si la IP está en la lista. |
| `unexpectIPs` | **IPs no esperadas** | Descartar respuestas con las IPs indicadas. |
| `skipFallback` | **Omitir Fallback** | No usar este servidor como fallback. |
| `finalQuery` | **Consulta final** | Marca el servidor como final en la cadena. |
| `timeoutMs` | **Tiempo de espera (ms)** | Tiempo de espera de la solicitud al servidor. |

#### Hosts (registros estáticos)

Grupo **Hosts** (`dns.hosts`). Botón **Agregar Host**; estado vacío **Hosts no definidos**. Campos: dominio (marcador de posición: *«Dominio (p. ej. domain:example.com)»*) y valores (marcador de posición: *«IP o dominio — introduzca y pulse Enter»*).

#### Logs de DNS

Véase [11.10](#1110-logs-y-estadísticas-stats-metrics): indicador **Logs DNS** (`dnsLog`) en la sección de registro.

### 11.7. Fake DNS

Sección `fakedns`. Botones: **Crear Fake DNS**, **Editar Fake DNS**.

| Campo | Etiqueta | Descripción |
|---|---|---|
| `ipPool` | **Subred del pool de IPs** | Rango CIDR del que se asignan IPs ficticias (por ejemplo `198.18.0.0/15`). |
| `poolSize` | **Tamaño del pool** | Cuántas direcciones mantener en el pool circular. |

Fake DNS se usa junto con el sniffing en el inbound: el núcleo entrega al cliente una IP ficticia, recuerda la correspondencia dominio↔IP y restaura el dominio al enrutar. Para que Fake DNS funcione, el servidor DNS con la dirección `fakedns` debe agregarse a la lista de servidores DNS.

**Ejemplo: combinación Fake DNS + servidor DNS.** Primero se define el pool de direcciones ficticias, luego se agrega el servidor DNS `fakedns` para que las solicitudes de dominio reciban IPs de este pool:

```json
"fakedns": [
  { "ipPool": "198.18.0.0/15", "poolSize": 65535 }
],
"dns": {
  "servers": [
    { "address": "fakedns", "domains": ["geosite:geolocation-!cn"] },
    "1.1.1.1"
  ]
}
```

Además, en el inbound hay que activar el sniffing con `destOverride: ["fakedns"]`, de lo contrario el núcleo no tendrá de dónde obtener el dominio real para la restauración.

### 11.8. WireGuard / WARP / NordVPN

#### Campos WireGuard (`wireguard`)

| Campo | Etiqueta | Descripción |
|---|---|---|
| `secretKey` | **Clave secreta** | Clave privada de la interfaz local. |
| `publicKey` | **Clave pública** | Clave pública del peer. |
| `psk` | **Clave compartida** | PreShared Key (opcional). |
| `allowedIPs` | **Direcciones IP permitidas** | Rangos enrutados al túnel. |
| `endpoint` | **Punto de conexión** | `host:port` del peer. |
| `domainStrategy` | **Estrategia de dominio** | Estrategia de resolución para el outbound WireGuard. |

#### Cloudflare WARP (`warp`)

La integración usa la API `https://api.cloudflareclient.com/v0a4005` (client-version `a-6.30-3596`). Acciones del controlador (`/xray/warp/:action`): `config`, `reg`, `license`, `data`, `del`.

Paso a paso:

1. **Crear cuenta WARP** → `reg`: el panel genera/acepta claves privada (`privateKey`) y pública (`publicKey`), registra el dispositivo en Cloudflare y guarda `access_token`, `device_id`, `license_key`, `private_key` (así como `client_id`) en la configuración `warp`.
2. **Clave de licencia WARP / WARP+** → `license`: instalación de la clave WARP+ de 26 caracteres (marcador de posición: *«Clave WARP+ de 26 caracteres»*). En caso de error: *«No se pudo establecer la licencia WARP.»* Si la configuración aún no se ha obtenido: *«Primero obtenga la configuración de WARP.»*
3. **Información de la cuenta**: **Nombre del dispositivo**, **Modelo del dispositivo**, **Dispositivo activado**, **Tipo de cuenta**, **Rol**, **WARP+ data**, **Cuota**, **Uso**.
4. **Agregar saliente** — crea un outbound WireGuard con las claves y el endpoint de Cloudflare obtenidos.
5. **Eliminar cuenta** → `del`: borra los datos WARP guardados.

#### NordVPN (`nord` / `nordvpn`)

La integración usa NordLynx (= WireGuard). Acciones del controlador (`/xray/nord/:action`): `countries`, `servers`, `reg`, `setKey`, `data`, `del`.

Paso a paso:

1. **Token de acceso** → `reg`: el panel solicita a `api.nordvpn.com` las credenciales NordLynx y extrae `nordlynx_private_key`. Guarda `private_key` y `token` en la configuración `nord`. Alternativa — `setKey`: introducir la **Clave privada** directamente (no puede estar vacía).
2. **País** → `countries` carga la lista de países; **Ciudad** (o **Todas las ciudades**).
3. **Servidor** → `servers` carga los servidores del país seleccionado (`countryId` se valida como número — protección contra inyecciones). Filtro: solo se muestran los servidores con **Carga** > 7%. Si no hay servidores: *«No se encontraron servidores para el país seleccionado»*. Si el servidor no tiene clave pública NordLynx: *«El servidor seleccionado no reporta clave pública NordLynx.»*
4. Creación/actualización del saliente: notificaciones *«Saliente NordVPN agregado»* / *«Saliente NordVPN actualizado»*.

#### Prioridad IPv4 y TUN en espacio de usuario

Los outbounds WireGuard generados por los asistentes de WARP y NordVPN usan `domainStrategy: "ForceIPv4v6"` (prioridad IPv4 con retroceso a IPv6 en hosts solo con v6) en lugar de `ForceIP` — esto elimina el «bloqueo» del handshake en hosts con IPv6 configurado a medias, cuando se selecciona el registro AAAA del endpoint de Cloudflare. Además, para ellos se activa el TUN en espacio de usuario (`noKernelTun: true`) en lugar del TUN del kernel: este último requiere permisos y enrutamiento fwmark, y falla silenciosamente en muchos VPS, mientras que la verificación de conexión integrada del panel siempre prueba a través del TUN en espacio de usuario — ahora el tráfico real y la verificación van por el mismo camino. El cambio solo afecta a los outbounds recién agregados o restablecidos; las plantillas ya guardadas conservan su configuración.

### 11.9. Reverse-proxy y TUN

#### Reverse (reverse-proxy)

Sección `reverse` de la configuración de Xray. En el formulario de outbound hay un interruptor al tipo **Reverse-proxy**. Botones: **Crear reverse-proxy**, **Editar reverse-proxy**.

| Campo | Etiqueta | Descripción |
|---|---|---|
| Tipo | **Tipo** | **Bridge** o **Portal** — dos roles del reverse-proxy de Xray. |
| Dominio | **Dominio** | Dominio de etiqueta de servicio para el par bridge↔portal. |
| Etiqueta / Conexión | **Etiqueta** / **Conexión** | Etiquetas para la vinculación de bridge y portal. |
| Reverse Tag | **Etiqueta de reverse-proxy** | Pista: *«Etiqueta de la conexión saliente para el reverse-proxy VLESS simple. Deje vacío para desactivar.»* Marcador de posición: *«etiqueta del saliente (vacío = desactivado)»*. Implementa el reverse VLESS simplificado. |

En el formulario de outbound también están presentes los campos de flujo inverso: **Sniffing inverso**, **Workers**, **Reservado**, **Intervalo mínimo de carga (ms)**, **Tamaño máximo de carga (bytes)**.

#### TUN (`tun`)

| Campo | Etiqueta | Descripción | Por defecto |
|---|---|---|---|
| name | — | *«Nombre de la interfaz TUN.»* | **`xray0`** |
| mtu | — | *«Unidad máxima de transmisión. Tamaño máximo de los paquetes de datos.»* | **1500** |
| `userLevel` | **Nivel de usuario** | *«Todas las conexiones establecidas a través de este flujo entrante usarán este nivel de usuario.»* | **0** |

### 11.10. Logs y estadísticas (Stats, metrics)

#### Log (`log`)

Pista: *«Los logs pueden ralentizar el servidor. ¡Active solo los tipos de logs que necesite!»* Sección `log` de la plantilla de referencia: `access: "none"`, `error: ""`, `loglevel: "warning"`, `dnsLog: false`, `maskAddress: ""`.

| Campo | Etiqueta | JSON | Descripción | Por defecto |
|---|---|---|---|---|
| `logLevel` | **Nivel de logs** | `loglevel` | *«Nivel de registro para los logs de errores…»* Valores: `debug`, `info`, `warning`, `error`, `none`. | **`warning`** |
| `accessLog` | **Logs de acceso** | `access` | *«Ruta al archivo de log de acceso. El valor especial «none» desactiva los logs de acceso.»* | **`none`** |
| `errorLog` | **Logs de errores** | `error` | *«Ruta al archivo de logs de errores. El valor especial «none» desactiva los logs de errores.»* | **`""`** (por defecto) |
| `dnsLog` | **Logs DNS** | `dnsLog` | *«Activar los logs de solicitudes DNS»* | **false** |
| `maskAddress` | **Enmascarar dirección** | `maskAddress` | *«Al activarse, la dirección IP real se reemplaza por una dirección de máscara en los logs.»* | **`""`** (desact.) |

#### Estadísticas (`stats` / `policy`)

Grupo **Estadísticas**. Activa los contadores en `policy.system` y `policy.levels`. En la plantilla de referencia: `statsInboundUplink: true`, `statsInboundDownlink: true`, `statsOutboundUplink: false`, `statsOutboundDownlink: false`; para el nivel `0` — `statsUserUplink: true`, `statsUserDownlink: true`.

| Campo | Etiqueta | Descripción | Por defecto |
|---|---|---|---|
| `statsInboundUplink` | **Estadísticas de uplink entrante** | *«Activa la recopilación de estadísticas del tráfico saliente de todos los proxies entrantes.»* | **true** |
| `statsInboundDownlink` | **Estadísticas de downlink entrante** | *«Activa la recopilación de estadísticas del tráfico entrante de todos los proxies entrantes.»* | **true** |
| `statsOutboundUplink` | **Estadísticas de uplink saliente** | *«Activa la recopilación de estadísticas del tráfico saliente de todos los proxies salientes.»* | **false** |
| `statsOutboundDownlink` | **Estadísticas de downlink saliente** | *«Activa la recopilación de estadísticas del tráfico entrante de todos los proxies salientes.»* | **false** |

> Las estadísticas de clientes e inbounds (uplink/downlink) son la base para mostrar el tráfico en el panel y en los clientes; no se recomienda desactivarlas. Las estadísticas de outbounds están desactivadas por defecto y solo son necesarias si se monitorea el tráfico por etiquetas de salientes.

#### Metrics

En la plantilla de referencia hay una sección `metrics` (`listen: "127.0.0.1:11111"`, `tag: "metrics_out"`) y la API correspondiente `metrics_out`. El panel usa este listener para recopilar métricas e instantáneas del observatory: analiza `metrics.listen` de la plantilla, consulta `/debug/vars` y agrega el historial de latencias por etiquetas. Si cambia la dirección/puerto de `metrics.listen`, el panel accederá a la nueva dirección; eliminar la sección `metrics` desactivará la recopilación de gráficos del observatory.

> La prueba de outbound en modo HTTP levanta una **instancia temporal separada** de Xray con su propio listener `metrics` en un puerto aleatorio — no es el mismo listener que en la configuración principal.

### 11.11. Guardado, reinicio y transformaciones automáticas

#### Botones

| Botón | Acción |
|---|---|
| **Guardar** | `POST /xray/update`: valida y guarda la plantilla + `outboundTestUrl`. |
| **Reiniciar Xray** | Recarga el servicio con la configuración guardada. Confirmación: *«¿Reiniciar xray?»* / *«Recarga el servicio xray con la configuración guardada.»* |

Notificaciones: éxito — *«Xray reiniciado correctamente»*, *«Xray detenido correctamente»*; errores — *«Se produjo un error al reiniciar Xray.»*, *«Se produjo un error al detener Xray.»* La ventana **Salida del reinicio de Xray** muestra la salida de diagnóstico del núcleo.

#### Aplicación en caliente de cambios (sin reinicio completo)

Los cambios en inbounds, outbounds y reglas de enrutamiento se aplican «en vivo»: al hacer clic en **Guardar**, el panel calcula la diferencia entre la configuración antigua y la nueva, y aplica solo las partes modificadas a través de la API gRPC de Xray (HandlerService/RoutingService), sin reiniciar el proceso. El reinicio completo se ejecuta automáticamente solo cuando cambian secciones sin API de recarga en caliente (`log`, `dns`, `policy`, `observatory`, etc.). Por eso en la página de Xray no hace falta un botón «Reiniciar» separado — **Guardar** aplica los cambios por sí mismo. El reinicio del núcleo cuando es necesario sigue ejecutándose automáticamente (véase también la recarga automática con actualizaciones de suscripciones y la rotación de WARP).

#### Restaurar la plantilla predeterminada

El endpoint `GET /xray/getDefaultJsonConfig` devuelve la plantilla de referencia (`config.json`, integrada en el binario). Se puede usar para restablecer la configuración a los valores de fábrica.

#### Transformaciones automáticas al guardar

Al guardar la configuración de Xray, el panel realiza (en este orden):

1. **Eliminación de envolturas** — elimina envolturas del tipo `{ "xraySetting": <config>, "inboundTags": …, "outboundTestUrl": … }`, si accidentalmente se incluyeron en el valor (de lo contrario las capas se acumularían con cada guardado). Se eliminan hasta 8 capas.
2. **Verificación de la configuración** — el JSON se analiza en una estructura de configuración de Xray; en caso de error — rechazo con *«xray template config invalid»*.
3. **Garantía de la regla de estadísticas** — la regla `inboundTag: ["api"] → outboundTag: "api"` se eleva forzosamente a la posición 0 en `routing.rules` (o se agrega si está ausente). Esto garantiza que la solicitud de estadísticas gRPC del panel no sea interceptada por una regla catch-all superior (de lo contrario los clientes pueden aparecer offline con tráfico cero mientras el proxy funciona).

> Por el punto 3, no intente eliminar o mover la regla `api → api` — el panel la restaurará en su lugar en el próximo guardado. Es la infraestructura de servicio de estadísticas, no una ruta de usuario.

### 11.12. Outbound de suscripción (con actualización automática)

A partir de la versión 3.3.0, el panel puede importar `outbound`s directamente desde una URL de suscripción — el mismo formato que ofrecen los proveedores de VPN para las aplicaciones cliente. Las suscripciones se releen periódicamente en segundo plano, por lo que el conjunto de `outbound`s en el servidor se mantiene actualizado sin edición manual de la plantilla de configuración.

En la interfaz el apartado se llama **«Suscripciones de salientes»**, descripción: «Importar salientes desde URLs de suscripción remotas (vmess/vless/trojan/ss/...). Las etiquetas permanecen sin cambios para usarlas en balanceadores y reglas de enrutamiento. La actualización se realiza automáticamente.» El apartado está en la página Xray, encima del panel de configuración de `outbound`s.

#### Cómo funciona

Las suscripciones se almacenan por separado de la plantilla de configuración de Xray. La plantilla **nunca se sobreescribe**: los `outbound`s obtenidos de las suscripciones se agregan a la configuración final al vuelo cada vez que se genera el config de Xray.

#### Agregar una suscripción

En el formulario «Agregar suscripción» están disponibles los siguientes campos:

| Campo | Clave | Por defecto | Función |
|------|------|--------------|------------|
| URL de suscripción | `url` | — (obligatorio) | Dirección de la suscripción. Marcador de posición: «https://... (lista de enlaces en base64)». Solo se acepta HTTP(S); la dirección se verifica por seguridad. |
| Nota | `remark` | vacío | Etiqueta arbitraria (marcador de posición «p. ej. nodos HK»). |
| Prefijo de etiqueta | `tagPrefix` | `subN-` | Prefijo con el que comienzan las etiquetas de los `outbound`s importados. Si se deja vacío, el panel asignará automáticamente el número libre más pequeño del tipo `sub1-`, `sub2-`, etc. |
| Intervalo de actualización | `updateInterval` | 600 segundos (10 minutos) | Con qué frecuencia se relee la suscripción. En la interfaz se especifica en horas/minutos. |
| Activada | `enabled` | sí (`true`) | Solo las suscripciones activadas se incluyen en el config y se actualizan automáticamente. |
| Permitir direcciones privadas | `allowPrivate` | no (`false`) | Permite URLs en localhost, LAN e IPs privadas. Desactivado por defecto para protección contra SSRF — actívelo solo para fuentes locales de confianza. |
| Antes de los salientes manuales | `prepend` | no (`false`) | Si se activa, los `outbound`s de esta suscripción se colocan **antes** de los `outbound`s manuales de la plantilla, y uno de ellos puede convertirse en el `outbound` predeterminado. De lo contrario se agregan **después**. |

El botón **«Vista previa»** (`POST /outbound-subs/parse`) permite descargar y analizar la URL antes de guardar y ver qué `outbound`s y etiquetas se obtendrán; nada se escribe en la base de datos en este momento. Si no se reconoce nada por la URL, se muestra «No se encontraron salientes en esta URL.»

El orden de varias suscripciones en la lista general de `outbound`s se define por prioridad (`priority`) y se cambia con las flechas arriba/abajo (`POST /outbound-subs/:id/move`).

#### Qué formatos de suscripción se aceptan

El cuerpo de la respuesta por URL se procesa así:

- El contenido se intenta primero como **base64** (variantes estándar y URL-safe, con autocompletar el padding y eliminar espacios/saltos de línea). Si es base64 — se decodifica; de lo contrario se toma tal cual.
- Luego el cuerpo se divide en líneas. Cada línea no vacía que no comience con `#` se analiza como un enlace. Las líneas no reconocidas (comentarios, protocolos no soportados) se omiten silenciosamente.
- Esquemas de enlace soportados: `vmess://`, `vless://`, `trojan://`, `ss://` (Shadowsocks), `hysteria2://` / `hy2://`, `wireguard://` / `wg://`.

Es decir, es compatible con la suscripción estándar del tipo «lista de enlaces codificada en base64», como la de la mayoría de los proveedores.

#### Etiquetas estables

Para cada enlace se calcula una «identidad» estable (núcleo del URI sin el fragmento de nota; para vmess — el JSON interno sin el campo `ps`). La correspondencia «identidad → etiqueta» se conserva, y en la próxima actualización el mismo servidor recibe la misma etiqueta, aunque hayan cambiado la nota o los parámetros secundarios. Esto se hace específicamente para que los balanceadores y las reglas de enrutamiento sigan funcionando tras las actualizaciones:

- Una etiqueta exacta en el balanceador/regla seguirá apuntando al mismo servidor.
- Un selector de prefijo/wildcard (por ejemplo, `hk-*`) tomará automáticamente los nuevos servidores que la suscripción devuelva más tarde — esta es la forma recomendada de «suscribirse a un pool».
- Si un servidor desaparece de la suscripción, su etiqueta simplemente deja de aparecer en el arreglo final de `outbound`s; si el balanceador tiene `fallbackTag`, Xray lo usa.
- Si el proveedor cambió el UUID/host/credenciales del servidor, la identidad cambia — esto se considera un nuevo `outbound` con una nueva etiqueta.

Dentro de una misma descarga, las etiquetas se deduplicaron con el sufijo `-N`. Las etiquetas de suscripciones conservan caracteres no ASCII (por ejemplo, cirílico) y permanecen legibles: las letras y dígitos Unicode se conservan en el slug, y la puntuación se reemplaza por un guión — las etiquetas de nombres en cirílico ya no se reducen solo a números.

#### Cómo funciona la actualización automática

- La tarea de actualización de suscripciones en segundo plano se ejecuta según un calendario **cada 5 minutos**.
- En cada ejecución recorre todas las suscripciones activadas y actualiza solo las que han superado su propio intervalo: una suscripción se actualiza si aún no se ha actualizado ninguna vez, o si desde la última actualización ha transcurrido al menos su `updateInterval`. Así la tarea verifica las suscripciones con frecuencia, pero cada suscripción concreta se relee no más de una vez por su `updateInterval` (10 minutos por defecto). En la interfaz esto se refleja con la pista correspondiente.
- Actualización: la URL se vuelve a verificar por seguridad como pública (las direcciones privadas se bloquean si la suscripción no tiene `allowPrivate` activado), la solicitud va a través del cliente proxy del panel con el encabezado `User-Agent: 3x-ui-outbound-sub/1.0`. La cadena de redirecciones está limitada a 10 saltos, y cada salto también se verifica por privacidad (protección contra SSRF). Se espera HTTP 200; de lo contrario se registra un error.
- Tras el análisis exitoso, el resultado se guarda, se registra la hora de la última actualización y se borra el error. En caso de error, su texto es visible en la interfaz como «Último error», y los `outbound`s obtenidos anteriormente siguen siendo válidos.
- Si al menos una suscripción se actualizó realmente, la tarea marca Xray para reinicio y envía una invalidación de interfaz para que la UI cargue los nuevos `outbound`s. El reinicio real de Xray ocurre en el próximo ciclo de 30 segundos del gestor.

La actualización manual de una suscripción se realiza con el botón **«Actualizar ahora»** (`POST /outbound-subs/:id/refresh`); también marca Xray para reinicio. Agregar, modificar o eliminar una suscripción también activa el indicador de reinicio de Xray (al eliminar, sus `outbound`s desaparecen del config en el próximo reinicio). La interfaz indica: «Tras agregar o actualizar, reinicie Xray (o espere el próximo reinicio automático) para que los salientes queden activos.»

#### Cómo se incluye en el config de Xray

En cada generación de la configuración de Xray, los `outbound`s de suscripciones activas se dividen en dos grupos — `prepend` (indicador «Antes de los salientes manuales») y el resto — y se combinan con la plantilla: `[prepend de suscripciones] + [outbound`s de la plantilla] + [resto de suscripciones]`. Dentro de cada grupo, las suscripciones van por prioridad. Los `outbound`s manuales de la plantilla no se ven afectados; si por alguna razón el arreglo de `outbound`s de la plantilla no se puede analizar, los `outbound`s de suscripción no se mezclan en él (para no perder los manuales).

Los `outbound`s importados también se muestran en el propio panel de `outbound`s en un bloque separado **«De suscripciones de salientes (solo lectura)»** — no se pueden editar allí, la gestión es únicamente a través del apartado «Suscripciones de salientes».

### 11.13. Rotación de IP en WARP

En 3X-UI se puede levantar un outbound WARP — una conexión WireGuard saliente hacia Cloudflare WARP (etiqueta `warp` en el config de Xray). El panel registra por sí mismo en los servidores de Cloudflare una cuenta de dispositivo, obtiene las claves WireGuard y las direcciones, y las inserta en el outbound con la etiqueta `warp`. A través de este outbound, el tráfico sale a internet bajo la dirección IP de Cloudflare WARP. La novedad de la versión 3.3.0 es la posibilidad de cambiar esta IP saliente manualmente o según un calendario, sin recrear la cuenta WARP manualmente.

La gestión se encuentra en el apartado **Xray** en la tarjeta WARP (tras hacer clic en «Crear cuenta WARP» y obtener el config; hasta entonces las acciones no están disponibles — el panel indicará «Primero obtenga la configuración de WARP»).

#### Qué ocurre al cambiar la IP

El botón **«Cambiar IP»** inicia el cambio de IP. La lógica:

1. Se genera un nuevo par de claves WireGuard.
2. Con la nueva clave se vuelve a registrar el dispositivo WARP en los servidores de Cloudflare (nuevo `device_id`, `access_token`, direcciones y datos del peer).
3. Los nuevos datos se escriben en el outbound WARP del config de Xray: se actualizan `secretKey`, `address` (v4 `/32` y v6 `/128`), `reserved` (de `client_id`), así como `publicKey` y `endpoint` del peer.
4. Si se había establecido previamente una clave de licencia WARP+ (de al menos 26 caracteres), se reinstala automáticamente en la nueva cuenta. En caso de fallo, esto es solo una advertencia en los logs — el cambio de IP no se cancela.
5. Tras el cambio exitoso, Xray se marca como que requiere reinicio para que el nuevo outbound entre en vigor.

Cuando se produce el éxito, la interfaz muestra «¡La dirección IP de WARP se ha cambiado correctamente!».

#### Rotación automática según calendario

En la tarjeta WARP hay un interruptor **«Actualización automática de la dirección IP»** y el campo **«Intervalo (días)»**. Pista: «0 — desactivar. Cambia automáticamente la dirección IP.»

| Parámetro | Valor |
|---|---|
| Configuración en BD | `warpUpdateInterval` (entero, ≥ 0) |
| Valor por defecto | `0` (rotación automática desactivada) |
| Unidad de medida | días |
| `0` | desactiva el cambio automático |
| `> 0` | cambiar la IP cada N días |

Guardar el intervalo almacena `warpUpdateInterval`, y con un valor mayor que 0 restablece la «hora de la última actualización» al momento actual — de lo contrario el planificador cambiaría la IP en el siguiente ciclo.

El calendario lo ejecuta una tarea en segundo plano que se lanza una vez por hora — es decir, el panel verifica una vez por hora si es hora de rotar. Algoritmo de verificación:

- si el intervalo es ≤ 0 — no hace nada;
- si la «hora de la última actualización» es 0 (por ejemplo, el intervalo se configuró editando directamente la BD) — es la primera ejecución: la tarea solo registra la marca de tiempo base y NO cambia la IP inmediatamente;
- si desde la última actualización han transcurrido al menos `intervalo × 24 × 3600` segundos — se ejecuta el mismo cambio de IP, se actualiza la marca de tiempo y se programa el reinicio de Xray.

Detalle importante: el cambio manual con el botón «Cambiar IP» también restablece la marca de tiempo de la última actualización. Por eso, tras una rotación manual, el conteo del intervalo automático comienza de nuevo y el cambio programado no se ejecutará inmediatamente después.

#### «A través del proxy del panel»

> **Cambiado en 3.3.1.** La configuración separada «Proxy de red del panel» (`panelProxy`) fue eliminada. El tráfico saliente del propio panel (incluidas las solicitudes a la API de WARP) ahora se dirige a través del **outbound de tráfico del panel** seleccionado — un outbound de Xray o un balanceador (véase el apartado [13](#13-configuración-del-panel)). La descripción a continuación corresponde a versiones anteriores a 3.3.1.

Todas las solicitudes a la API de Cloudflare WARP (registro, obtención de config, instalación de licencia, cambio de IP) no van directamente, sino a través del cliente HTTP del panel con un tiempo de espera de 15 segundos. Este cliente respeta la configuración **«Proxy de red del panel»** (`panelProxy`) de la configuración del panel.

De la descripción de la configuración: el proxy enruta las propias solicitudes salientes del panel (actualizaciones de bases geo, verificaciones de versiones de Xray/panel, Telegram, y ahora también las solicitudes a WARP) — para eludir el filtrado del servidor. Se aceptan direcciones del tipo `socks5://` o `http(s)://`, por ejemplo el propio inbound SOCKS de Xray local. Si el campo está vacío o el proxy está configurado incorrectamente — se usa la conexión directa (el comportamiento no se rompe).

Utilidad para WARP: si el servidor no puede alcanzar directamente `api.cloudflareclient.com`, el registro y la rotación antes fallaban. Ahora, al especificar en `panelProxy` un proxy funcional (incluyendo el propio inbound de Xray), se puede garantizar la disponibilidad de la API de WARP y el funcionamiento tanto del botón manual como de la rotación programada.

#### Cuándo es útil

- Cambio regular de la IP saliente para el outbound que sale a través de WARP — reduce el riesgo de bloqueos y rastreo por una sola dirección.
- «Refrescar» la IP manualmente si la dirección actual de Cloudflare está en listas negras o funciona lentamente.
- Servidores sin acceso directo a la API de Cloudflare WARP: enrutar las solicitudes a través de `panelProxy` hace que el registro y la rotación sean funcionales.

---

## 12. Nodos (multipanel, master/slave)

La sección **Nodos** convierte una instalación normal de 3X-UI en un **panel central (maestro)** que supervisa y gestiona de forma remota otros paneles 3X-UI (subordinados). Cada nodo es una instalación independiente de 3X-UI en su propio servidor; el maestro se conecta a ella a través de su propia API HTTP, consulta su estado y sincroniza en ella los inbounds y clientes que tienen asignados. Esta es la capacidad de **multipanel**: en lugar de entrar en cada panel por separado, se ven todos los servidores en una sola lista y se gestionan de forma centralizada.

Principio importante: **un nodo no es un agente, sino un panel 3X-UI completo.** El maestro no «instala» nada en él — simplemente se conecta a su API mediante un token. Eliminar un nodo de la lista solo detiene la supervisión; el panel remoto en sí no se ve afectado (aviso: «Esto detendrá la supervisión del nodo. El panel remoto en sí no se verá afectado»).

### 12.1. Resumen en la parte superior de la lista

Encima de la tabla de nodos se muestran contadores agregados:

| Campo | Descripción |
|---|---|
| Total de nodos | Número total de nodos en la lista. |
| En línea | Cuántos nodos tienen el estado `online`. |
| Fuera de línea | Cuántos nodos tienen el estado `offline`. |
| Latencia media | Latencia promedio (ping) a los nodos, en milisegundos. |

### 12.2. Añadir y editar un nodo

Los botones **Añadir nodo** y **Editar nodo** abren el formulario con los campos del nodo.

Son obligatorios (aviso: «El nombre, la dirección, el puerto y el token API son obligatorios») los campos **Nombre**, **Dirección**, **Puerto** y **API Токен**.

Al pulsar «Guardar» (tanto al añadir como al editar), el panel **primero verifica la accesibilidad del nodo** con un tiempo de espera de 6 segundos. Si el nodo no responde, el registro no se guarda y se muestra un error. Es decir, no se puede añadir un nodo que sea claramente inaccesible.

#### Campos del formulario

| Campo | Por defecto | Valores válidos | Descripción |
|---|---|---|---|
| Nombre | — (obligatorio) | cadena no vacía, **única** | Nombre interno del nodo. Se aplica unicidad a la columna de nombre — no se pueden crear dos nodos con el mismo nombre. Marcador de posición: `napr. de-frankfurt-1`. Al guardar, se recortan los espacios en los extremos. |
| Nota | vacío | cualquier cadena | Nota/descripción opcional del nodo. No afecta al funcionamiento. |
| Esquema | `https` | `http` / `https` | Protocolo de conexión al panel remoto. Si se deja vacío o se indica un valor no válido, la normalización establecerá `https`. Si el nodo responde por HTTP simple pero el esquema está en `https`, el panel devolverá una sugerencia clara: «the server speaks HTTP, not HTTPS; set the node scheme to http». |
| Dirección | — (obligatorio) | host o IP | Dirección del panel remoto. Marcador de posición: `panel.example.com ou 1.2.3.4`. La dirección se normaliza; por defecto, las direcciones privadas/locales están prohibidas como protección contra SSRF — véase «Permitir dirección privada». |
| Puerto | — (obligatorio) | entero **1–65535** | Puerto del panel web del nodo remoto. Los valores fuera del rango se rechazan («node port must be 1-65535»). |
| Ruta base | `/` | cadena de ruta | Ruta base (web base path) del panel remoto, si está configurada. Se normaliza: garantizado que empieza y termina con `/` (valor vacío → `/`). El panel añade `panel/api/server/status` al consultarla. |
| API Токен | — (obligatorio) | token del panel remoto | Token Bearer para acceder a la API del nodo. Se envía en el encabezado `Authorization: Bearer <token>`. Marcador de posición: «Token de la página de Ajustes del panel remoto». Aviso: «El panel remoto muestra su token API en la sección Ajustes → Token API». Es decir, el token debe crearse **en el propio nodo** (Ajustes → Token API) y luego pegarse aquí. |
| Habilitado | `true` | sí/no | Activa la supervisión y sincronización del nodo. Los nodos deshabilitados **no son consultados** por las tareas en segundo plano (el heartbeat y la sincronización de tráfico los omiten) y no participan en la actualización masiva del panel. |
| Permitir dirección privada | `false` | sí/no | Elimina la protección SSRF y permite conectarse al nodo mediante una dirección privada/local. Aviso: «Activar solo para nodos en red privada o VPN». Actívelo solo cuando el nodo esté realmente en una red privada o sea accesible a través de VPN. |

#### Obtención y regeneración del token en el lado del nodo

El token se obtiene en el panel remoto en la sección **Ajustes → Token API**. Allí también se puede regenerar: el botón **Generar token de nuevo** con advertencia: «Regenerar el token invalidará el token actual. Cualquier panel central que lo use perderá el acceso hasta que se actualice. ¿Continuar?». Tras la regeneración, el token antiguo en el panel maestro dejará de funcionar — es necesario actualizarlo en el formulario del nodo.

#### Conexión saliente (Connection outbound)

El campo **Connection outbound** (Conexión saliente, `outboundTag`) define cómo el tráfico de las solicitudes del maestro a la API de este nodo sale del servidor. Si se selecciona en él un tag de Xray-outbound, las solicitudes del panel al nodo no irán directamente, sino a través del outbound especificado; el panel añadirá automáticamente al la configuración en ejecución un bridge-inbound en loopback y aplicará el cambio en caliente, sin reinicio. Aviso: «Route this node's panel API traffic through the selected Xray outbound. A loopback bridge inbound is added to the running config automatically and applied live. Leave empty for a direct connection».

El selector funciona como la selección de outbound del panel: los tags se agrupan en **Outbounds** (salientes habituales) y **Balancers** (balanceadores); los outbounds blackhole se ocultan de la lista. El valor vacío (marcador de posición «Direct connection») = conexión directa al nodo.

#### Importar inbound (selección de inbounds a sincronizar)

El formulario del nodo tiene la configuración **Importar inbound** (`inboundSyncMode`) con dos modos: **Todos los inbound** (`all`, por defecto) y **Seleccionados** (`selected`). Por defecto, el maestro sincroniza en el nodo todos los inbounds que tienen seleccionado ese nodo; los nodos existentes continúan funcionando en modo «Todos los inbound».

En el modo **Seleccionados**, aparece bajo el campo una selección múltiple de tags de inbound. Pulse **Cargar inbound** — el maestro, usando los parámetros de conexión introducidos (aún no guardados), solicitará al nodo la lista de sus inbounds (endpoint `POST /panel/api/nodes/inbounds`) y mostrará sus tags; marque los que necesite. El panel sincronizará y desplegará en el nodo solo los tags marcados, mientras que el resto de los inbounds existentes directamente en el nodo permanecerán intactos — el maestro no los elimina ni los gestiona.

**Ejemplo: solicitar la lista de inbounds del nodo para importación selectiva.** En el cuerpo se pasan los parámetros de conexión aún no guardados; en la respuesta — los tags de los inbounds disponibles en el nodo:

```
POST /panel/api/nodes/inbounds
Content-Type: application/json

{ "name": "de-fra-1", "scheme": "https", "address": "node1.example.com",
  "port": 2053, "basePath": "/", "apiToken": "abcdef..." }
```

### 12.3. Verificación TLS (para nodos https)

El grupo de campos define cómo el maestro verifica el certificado HTTPS del nodo. Estas configuraciones **solo son relevantes para el esquema `https`**; para nodos `http` se ignoran.

**Verificación TLS** — lista desplegable, aviso: «Cómo verifica el panel el certificado HTTPS del nodo. Fijación o Omisión — para certificados autofirmados (solo nodos https)».

| Modo | Valor | Por defecto | Descripción |
|---|---|---|---|
| Verificar (CA estándar) | `verify` | sí (default) | Verificación normal de la cadena de certificados con CA de confianza. Adecuado para nodos con certificado público/Let's Encrypt. También se usa para todos los nodos `http`. |
| Fijar certificado (SHA-256) | `pin` | — | No se verifica la cadena de CA, pero el SHA-256 del certificado hoja del nodo se compara con la huella digital guardada (comparación en tiempo constante). Mantiene la protección contra MITM para certificados **autofirmados**. Requiere rellenar el campo de huella digital. |
| Omitir verificación | `skip` | — | La verificación del certificado se desactiva completamente. Advertencia: «Omitir la verificación elimina la protección contra ataques de intermediario — el token API puede ser interceptado. Es mejor fijar el certificado». |

A los tres modos anteriores, en 3.4.0 se añadió un cuarto — **Mutual TLS (client certificate)** (`mtls`), disponible, como los demás, solo para el esquema `https`.

| Modo | Valor | Por defecto | Descripción |
|---|---|---|---|
| Mutual TLS (certificado de cliente) | `mtls` | — | Además de verificar el certificado del nodo, el maestro también se autentica ante el nodo con un **certificado de cliente** emitido por su propio CA. Para el nodo en este modo, **el token API se vuelve opcional** — el nodo reconoce al maestro por el certificado. Al seleccionar el modo se muestra el aviso: «This node authenticates the panel with a client certificate. Copy this panel's CA from the Node mTLS section onto the node, set its Trusted parent CA, then restart it». |

Para habilitar el TLS mutuo para un nodo: en el lado del nodo configure el modo **Mutual TLS**, copie el CA del panel gestor desde la sección **Node mTLS** (véase más abajo), regístrelo en el nodo como **CA padre de confianza** y reinicie el nodo.

Si se selecciona cualquier valor distinto de `skip`, `pin` o `mtls`, la normalización establecerá `verify` de forma forzada.

#### Fijación de certificado

Al seleccionar **Fijar certificado** aparecen:

- **SHA-256 del certificado fijado** — campo de entrada. Se acepta la huella digital en **base64** (formato `pinnedPeerCertSha256` de Xray) o en **hex** con o sin dos puntos (estilo `openssl -fingerprint`). Aviso: «SHA-256 del certificado del nodo en base64 o hex. Pulse «Obtener» para leerlo del nodo ahora». Marcador de posición: «SHA-256 en base64 o hex». Al seleccionar `pin`, una huella digital vacía o incorrecta provoca un error de validación al guardar.

**Ejemplo: la misma huella digital en dos formatos.** El campo acepta cualquiera de las variantes — ambas representan el mismo certificado:

```
# base64 (формат pinnedPeerCertSha256 из Xray)
6O7TNg3l2k0pq8R1sT2uV3wX4yZ5a6B7c8D9e0F1g2=

# hex с двоеточиями (стиль openssl x509 -fingerprint -sha256)
E8:E2:D3:60:DE:5D:9A:4D:29:AB:CF:11:B2:7C:34:...
```

Si la huella digital aún no se conoce, pulse **Obtener** — el maestro la leerá del nodo por HTTPS y la introducirá en el campo.
- Botón **Obtener** — se conecta al nodo por HTTPS sin verificar el certificado y lee el SHA-256 del certificado hoja actual (endpoint `POST /certFingerprint`), introduciéndolo en el campo. Tras el éxito — «Certificado actual del nodo obtenido»; en caso de fallo — «No se pudo obtener el certificado». Solo disponible para nodos https.

#### Node mTLS (autenticación TLS mutua entre paneles)

En la página **Nodos** hay una sección separada **Node mTLS** — configuración de la autenticación TLS mutua, que añade al token API un segundo factor (certificado de cliente) para las llamadas «panel → nodo». El TLS mutuo se habilita opcionalmente; si los campos de la sección están vacíos, los nodos funcionan según el esquema anterior — **solo con el token API** (aviso: «Mutual TLS adds a client-certificate factor on top of the API token for node-to-node calls. It is opt-in: leave it empty to keep token-only auth»). La sección tiene dos operaciones:

- **Copiar CA de este panel** (`POST /panel/api/nodes/mtls/ca`) — copia el certificado raíz (CA) de este panel al portapapeles. Este CA debe transferirse a los nodos gestionados para que confíen en el certificado de cliente del panel; en los propios nodos se establece luego el modo de verificación TLS **Mutual TLS** (aviso: «Hand this CA to the nodes this panel manages, then set their TLS verification to Mutual TLS»). Tras copiar — «CA certificate copied to clipboard».
- **CA padre de confianza** (`Trusted parent CA`, `POST /panel/api/nodes/mtls/trustCA`) — campo que se usa cuando este panel actúa como nodo para un panel superior (gestor). Pegue aquí el CA del panel gestor para exigirle un certificado de cliente y pulse **Save trust CA**. El cambio requiere **reiniciar el panel** (aviso: «When this panel is itself a node, paste the managing panel's CA here to require its client certificate. Restart the panel to apply»).

### 12.4. Qué se muestra por cada nodo

Columnas de la tabla y campos de la tarjeta del nodo (estado observado, se rellena en cada consulta heartbeat):

| Campo | Descripción |
|---|---|
| Estado | `online` / `offline` / `unknown` — véase más abajo. |
| CPU | Carga del procesador del servidor remoto en porcentaje. |
| Memoria | Uso de RAM en porcentaje (calculado como `current/total*100`). |
| Tiempo de actividad | Tiempo de funcionamiento continuo del servidor (en segundos). |
| Latencia | Tiempo de respuesta del nodo a la última consulta (ms). |
| Último ping | Hora del último heartbeat exitoso (segundos unix; `0` = «nunca»; un valor reciente se muestra como «justo ahora»). |
| Versión Xray | Versión del Xray-core en ejecución en el nodo. |
| Versión del panel | Versión de 3X-UI en el nodo — se compara con la actual para el indicador de actualización. |
| (inbounds) | Cuántos inbounds están físicamente alojados en este nodo. |
| (clientes) | Número de clientes en los inbounds del nodo. |
| (en línea) | Cuántos clientes del nodo están actualmente conectados. |
| (agotados) | Cuántos clientes del nodo **han expirado o agotado el límite de tráfico**. Los clientes deshabilitados manualmente no se incluyen en este contador. |
| (velocidad) | Velocidad de transferencia actual (en tiempo real) en los inbounds alojados en el nodo. |

Los contadores de inbounds/clientes/en línea se atribuyen al nodo por su GUID estable (`panelGuid`), no por su id local — para que un cliente en un subnodo se contabilice bajo ese subnodo y no bajo el nodo intermedio a través del cual se sincroniza.

Para los inbounds alojados en el nodo, la página muestra clientes en línea, contadores y **velocidad de transferencia actual**. La atribución por GUID estable diferencia correctamente también los nodos «clonados» con el mismo `panelGuid`.

#### Estados del nodo

| Estado | Cuándo se establece |
|---|---|
| `online` | El nodo respondió `success=true` a la consulta `panel/api/server/status`. |
| `offline` | El nodo no respondió, devolvió un error HTTP, `success=false` o una respuesta no reconocida. |
| `unknown` | Valor inicial, mientras el nodo aún no ha sido consultado ninguna vez. |

Cuando una consulta falla, el texto del error se guarda y se muestra con una formulación clara, lo que ayuda a diagnosticar la causa del estado «offline».

### 12.5. Acciones sobre el nodo

- **Probar conexión** (`POST /test`) — en el formulario del nodo, verifica la conectividad usando los parámetros introducidos (aún no guardados) con un tiempo de espera de 6 s. Resultado: «Conexión correcta ({ms} ms)» o «No se pudo conectar». Útil para depurar la dirección/puerto/token/TLS antes de guardar.
- **Comprobar ahora** (botón «Comprobar ahora», `POST /probe/:id`) — consulta no planificada de un nodo ya guardado; actualiza inmediatamente el estado y las métricas (CPU/memoria/tiempo de actividad/latencia/versiones) y registra el heartbeat. En caso de fallo — «La comprobación falló».

**Ejemplo: verificar y consultar un nodo a través de la API del maestro.** «Probar conexión» prueba los parámetros aún no guardados del formulario:

```
POST /panel/api/nodes/test
Content-Type: application/json

{ "scheme": "https", "address": "de-frankfurt-1.example.com", "port": 2053,
  "basePath": "/", "apiToken": "eyJhbGci...", "tlsMode": "verify" }
```

Consulta no planificada de un nodo ya guardado con id 7:

```
POST /panel/api/nodes/probe/7
```
- **Actualizar panel** (`POST /updatePanel` con cuerpo `{ids:[…]}`) — inicia en el nodo su actualizador integrado: el nodo descarga la última versión de 3X-UI y se reinicia con ella. El botón **Actualizar seleccionados ({count})** lo realiza para varios nodos marcados a la vez. Junto al nodo se muestra el indicador: **Actualización disponible** o **Al día**, basándose en la comparación de la versión del panel del nodo con la más reciente.

**Ejemplo: actualizar varios nodos con una sola solicitud.** En el cuerpo se pasan los id de los nodos marcados; solo se actualizarán los habilitados y `online`, el resto se devolverán como omitidos.

```
POST /panel/api/nodes/updatePanel
Content-Type: application/json

{ "ids": [3, 7, 12] }
```

Respuesta del tipo «Actualización iniciada en 2 nodos, 1 fallido»: el nodo 12, por ejemplo, podía estar offline y por eso fue omitido.
  - Confirmación: «¿Actualizar {count} nodos a la última versión? Cada nodo seleccionado descargará la última versión y se reiniciará. Solo se actualizan los nodos habilitados en línea».
  - **Solo se actualizan los nodos habilitados en estado `online`.** Un nodo deshabilitado aparece en los resultados como «node is disabled», offline — como «node is offline». Resultado: «Actualización iniciada en {ok} nodos, {failed} fallidos». Si no se selecciona ningún nodo adecuado — «Seleccione al menos un nodo habilitado en línea».

En el diálogo de confirmación de actualización (tanto para un solo nodo como para la masiva) hay una casilla **Actualizar al canal de desarrollo (último commit)**. Si se marca, los nodos seleccionados instalarán la compilación continua dev-latest (último commit de la rama main) en lugar del lanzamiento estable; con la casilla desmarcada, el nodo se actualiza por su canal habitual. Con la casilla activada, aparece debajo una advertencia: «Las compilaciones de desarrollo siguen cada commit en main y no son versiones estables — no hay reversión automática». El flag dev se pasa a través de `POST /panel/api/nodes/updatePanel` al nodo, y este inicia la actualización precisamente por el canal dev.
- **Set Cert from Panel** (auxiliar, `GET /webCert/:id`) — al crear un inbound en el nodo, permite insertar las rutas al certificado TLS **propio** del nodo (no del panel central), para que los archivos existan precisamente en el nodo. Requiere que el nodo esté habilitado y sea accesible.
- **Eliminar nodo** (`POST /del/:id`) — confirmación: «¿Eliminar el nodo "{name}"? Esto detendrá la supervisión del nodo. El panel remoto en sí no se verá afectado». Elimina el registro del nodo y sus estadísticas de tráfico acumuladas; el panel remoto continúa funcionando con normalidad. **Un nodo solo puede eliminarse después de que se le hayan desasignado todos los inbounds.** Si todavía hay al menos un inbound vinculado al nodo (mediante `node_id`), el panel rechazará la eliminación con un error del tipo «cannot delete node: N inbound(s) still attached to it; detach or delete them first» — primero desasigne o elimine esos inbounds y luego elimine el nodo. Esto evita inbounds «huérfanos» con una referencia pendiente al nodo eliminado.

### 12.6. Historial de métricas

El botón/gráfico de historial accede a `GET /history/:id/:metric/:bucket`. Las métricas disponibles son: **`cpu`** y **`mem`** — se acumulan en cada heartbeat exitoso. El tamaño del intervalo de agregación (`bucket`, en segundos) está limitado a una lista blanca:

**Ejemplo: solicitud de historial.** Gráfico de carga CPU del nodo 7 con agregación por intervalos de 60 segundos (se devuelven hasta 60 puntos):

```
GET /panel/api/nodes/history/7/cpu/60
```

Para memoria y modo «tiempo real» (2 s) — respectivamente `…/7/mem/60` y `…/7/cpu/2`. Los valores fuera de la lista blanca se rechazan («invalid metric» / «invalid bucket»).

| Bucket (s) | Uso |
|---|---|
| 2 | Modo tiempo real |
| 30 | Intervalos de 30 s |
| 60 | Intervalos de 1 min |
| 120 | Intervalos de 2 min |
| 180 | Intervalos de 3 min |
| 300 | Intervalos de 5 min |

Se devuelven hasta 60 puntos. Las métricas o buckets no válidos se rechazan («invalid metric» / «invalid bucket»).

### 12.7. Cómo se sincronizan los inbounds y los clientes

Un inbound «pertenece» a un nodo mediante el campo `node_id` (en el editor de inbound se selecciona el nodo):

**Ejemplo: token en el formulario del nodo.** El token se obtiene en el panel hijo (Ajustes → API Токен) y se pega en el campo **API Токен** del maestro. En cada consulta el maestro lo envía en el encabezado:

```
GET https://panel.example.com:2053/panel/api/server/status
Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.abc123...
```

Si en el panel hijo se ha configurado una **ruta base** (web base path), por ejemplo `/secret/`, el maestro la añadirá automáticamente antes de `panel/api/server/status` → `https://panel.example.com:2053/secret/panel/api/server/status`.

1. **Despliegue de configuración (reconciliación).** Ante cualquier cambio en un inbound/cliente vinculado a un nodo, el nodo se marca como «sucio». La tarea en segundo plano para cada nodo habilitado **en estado `online`** despliega en el nodo sus inbounds (por `node_id`) si hay cambios, y luego restablece el indicador de estado «sucio». Un nodo deshabilitado, offline o «sucio» se considera «pendiente» — el despliegue en él se pospone hasta que se restaure la conexión.
2. **Recolección de tráfico.** La misma tarea solicita al nodo una instantánea del tráfico y la fusiona con las estadísticas locales. Basándose en el tráfico fusionado, se verifica el agotamiento de límites/plazos y, si es necesario, se deshabilitan clientes; el contador «agotados» por nodo refleja precisamente esto. Si el nodo no está disponible, sus clientes en línea se borran.

   Para un cliente vinculado simultáneamente a varios paneles, el master en la misma tarea distribuye adicionalmente a los nodos el consumo de tráfico **total de todos los paneles** de ese cliente (en una tabla separada en el nodo, clave — GUID del maestro; se sobreescribe en cada envío, por lo que el restablecimiento en el lado del maestro también se propaga). En el nodo, en el tráfico del cliente se muestra el mayor de los dos valores — el local o el recibido —, y al superar la cuota total el cliente se deshabilita **localmente en el propio nodo** (mediante el mismo mecanismo de reinicio de Xray al deshabilitar automáticamente, que interrumpe las conexiones ya establecidas). Esto elimina la situación en que el nodo solo veía su parte del tráfico, subestimaba el consumo y seguía atendiendo a un cliente que ya había agotado el límite total. Al restablecer el tráfico, renovar automáticamente o eliminar el cliente, los contadores enviados se limpian.

   En la **primera** sincronización de un inbound alojado en el nodo (añadir un nuevo nodo o reimportar un inbound), el maestro inicializa los contadores de tráfico de los clientes con los valores reales del nodo. Antes, en esta situación, el contador total del inbound se transfería correctamente, pero los contadores individuales de los clientes se ponían a cero, y el maestro subestimaba el consumo de los clientes en todo el historial acumulado antes de conectar el nodo. Ahora, si el inbound se crea en la misma sincronización, la nueva fila `client_traffics` hereda el valor del contador del nodo (la línea base se establece igual a él, por lo que el siguiente delta es cero y el tráfico no se cuenta dos veces). El establecimiento del contador inicial solo se aplica al inbound creado en este mismo paso: un cliente que reaparece bajo un inbound ya existente sigue empezando desde cero (protección contra tráfico «fantasma»), y un cliente recién eliminado no «resucita» al recrear su inbound.
3. **Heartbeat.** Una tarea en segundo plano separada consulta periódicamente todos los nodos **habilitados** (con un límite de paralelismo) a través de `panel/api/server/status`, actualiza el estado/métricas/versiones y, si hay clientes web, distribuye el árbol de nodos actualizado por WebSocket.

### 12.8. Cadenas de nodos (subnodos / nodos transitivos)

La topología puede no ser plana: un nodo puede ser maestro de sus propios nodos. Dichos paneles subordinados se muestran para usted como **Subnodos** — son **proyecciones de solo lectura** obtenidas del nodo directo.

- Aviso: «Solo lectura: nodo subordinado accesible a través de {padre}. Gestiónelo desde el propio panel {padre}». Es decir, el subnodo no se puede editar, eliminar ni actualizar aquí — todas las operaciones con él se realizan desde el panel de su padre directo.
- La identidad del subnodo está determinada por su GUID; gracias a esto, los clientes en línea y los inbounds se atribuyen exactamente al nodo físico que los aloja, incluso en una cadena `Nodo1 → Nodo2 → Nodo3` (el maestro «avanza» un nivel más profundo a través de cada nodo directo).
- Si el nodo directo se vuelve inaccesible, su caché de subnodos se borra, y los subnodos desaparecen del árbol hasta que se restaure la conexión.

### 12.9. Nodos: novedades en 3.3.0

En la versión 3.3.0, la sección **Nodos** recibió tres mejoras notables: atribución correcta del tráfico y clientes en línea en topologías multinivel (multi-hop), sincronización de client-IP entre nodos e indicador de estado separado para el caso en que el panel del nodo está activo pero el núcleo Xray en él ha fallado.

#### 1. Multi-hop: atribución correcta del tráfico en la cadena de subnodos

Antes, los contadores (número de inbound, clientes en línea, agotados) se calculaban a nivel del nodo «directo». Si tenía una cadena del tipo `Maestro → Nodo1 → Nodo2 → Nodo3`, todo lo que físicamente residía en `Nodo2`/`Nodo3` se atribuía incorrectamente a `Nodo1`, a través del cual llegaba al maestro. En 3.3.0 la atribución se hace según la fuente real.

Cómo funciona:

- **Los subnodos se hacen visibles como filas separadas.** Cada panel publica la lista de sus nodos directos; solo se incluyen los nodos con `Guid` conocido — se necesita una identidad estable para atribuir el nodo a un «salto» superior. El maestro extrae periódicamente (desde la tarea heartbeat) estas listas y las almacena en caché, y luego añade a los nodos directos los subnodos «transitivos».
- **Los nodos transitivos son solo de lectura.** En la UI se marcan como **«Subnodo»** con el aviso: *«Solo lectura: nodo subordinado accesible a través de {padre}. Gestiónelo desde el propio panel {padre}.»* Dicha fila no tiene botones de gestión — el nodo se gestiona desde el panel de su padre inmediato.
- **Jerarquía mediante GUID.** El `ParentGuid` del nodo directo es el GUID del propio maestro; el del transitivo es el GUID de su nodo padre. Así se construye el árbol.
- **La fuente de verdad para los contadores es `origin_node_guid` en el inbound.** Es el `panelGuid` del nodo que físicamente mantiene ese inbound. Se establece durante la sincronización del inbound desde el nodo y **se conserva tal cual en los saltos posteriores**, por lo que un inbound profundamente anidado se atribuye al nodo real y no al intermedio. Con este GUID se recalculan los contadores de número de inbounds, clientes en línea y clientes agotados. Lógica de selección de clave:

  | Estado del inbound | A quién se atribuye |
  |---|---|
  | `origin_node_guid` definido | a este GUID (nodo fuente real) |
  | vacío, pero `node_id` definido | GUID sintético del nodo (compilación antigua, aún no informó su `panelGuid`) |
  | vacío y `node_id` vacío | GUID propio del maestro (inbound en Xray local) |

  Los clientes en línea también se agrupan por GUID, por lo que cada fila de nodo muestra solo los que están realmente conectados a él.

**Lo que ve el usuario:** en una topología plana (nodos directamente bajo el maestro) nada cambia — los contadores por GUID y por `id` coinciden. Pero en cuanto aparece una cadena de nodos, en la lista surgen filas-«Subnodo», y los números de inbound/en línea/agotados en cada nodo ahora reflejan exactamente su propia carga, no la suma de todo lo que pasó por él en tránsito.

#### 2. Sincronización de client-IP desde access.log entre nodos

El límite por IP (`limitIp` en el cliente) se basa en las direcciones que Xray escribe en su access.log. Antes, cada nodo solo veía las conexiones a sí mismo, por lo que la restricción «no más de N IP por cliente» no funcionaba en el clúster: el cliente podía conectarse a diferentes nodos y eludir el límite. En 3.3.0, las IP observadas se sincronizan en todo el clúster.

Cómo funciona:

- En cada nodo, una tarea en segundo plano analiza el access.log, extrayendo de cada línea la IP, el email del cliente y la marca de tiempo, y los almacena en una tabla local (una entrada por email, las IP se guardan como array JSON `{ip, timestamp}`). Las direcciones locales `127.0.0.1` y `::1` se descartan.
- La sincronización **cada 10 segundos** realiza un intercambio bidireccional por cada nodo habilitado en línea: extrae las IP del nodo y las fusiona en la tabla local, y luego envía al nodo el mapa resumen del maestro.
- La fusión combina las observaciones antiguas y las entrantes **sin doble conteo** de una IP vista en varios nodos, y **sin resucitar** registros obsoletos: se aplica el mismo umbral de antigüedad que en la tarea local — **30 minutos**. Para cada IP se guarda la marca de tiempo más reciente. Los registros de otros nodos reciben un nuevo id local (los espacios de id de los nodos son independientes); la inserción concurrente del mismo email está protegida contra duplicados.
- Al calcular el límite, se considera «activa» una IP que haya sido detectada en el escaneo local actual o que tenga una marca muy reciente de la base de datos sincronizada (**dentro de 2 minutos**). Precisamente esto hace que el límite funcione a escala de todo el clúster, incluso si la dirección fue detectada en otro nodo. Al superar el límite, las IP «activas» más antiguas se envían al registro fail2ban y las conexiones se interrumpen forzosamente (remove/re-add del cliente a través de la API de Xray).

**Lo que ve el usuario:** la restricción por número de IP ahora se aplica a todo el clúster, no a cada nodo por separado; en el panel, para cada cliente se ven las IP detectadas en cualquier nodo (dentro de una ventana de 30 minutos). No hay un botón/configuración separado para esto — la sincronización se realiza automáticamente en segundo plano, siempre que el acceso al access.log del nodo esté habilitado y disponible (para el propio límite también se requiere Fail2Ban en el nodo).

#### 3. Indicador de estado separado: el panel del nodo está en línea, pero Xray ha fallado

Antes, el estado del nodo era básicamente «en línea / fuera de línea». Si el panel del nodo respondía, el nodo se consideraba en línea — incluso cuando el núcleo Xray en él no funcionaba y los clientes de hecho no podían conectarse. En 3.3.0, la salud del panel y la salud del núcleo Xray se separan.

Cómo funciona:

- Al consultar el nodo, el maestro toma de la respuesta del `/panel/api/server/status` remoto los campos `xray.state` y `xray.errorMsg` y los guarda en el nodo. Estos campos se rellenan incluso cuando el ping del panel es exitoso pero el núcleo no está sano — precisamente para distinguir la disponibilidad del panel del estado de Xray.
- Valores de `xray.state`: `"running"` (en ejecución), `"stop"` (detenido), `"error"` (error).
- Estos valores se traducen en estados del nodo. A los habituales se añaden nuevos:

  | Clave de estado | Descripción | Cuándo se muestra |
  |---|---|---|
  | `online` | «En línea» | el panel responde, Xray está en ejecución (`running`) |
  | `offline` | «Fuera de línea» | el panel no es accesible / el ping falló |
  | `unknown` | «Desconocido» | el estado aún no está determinado |
  | `xrayError` | «Error de Xray» | el panel está en línea, pero el núcleo Xray está en estado `error` (hay `errorMsg`) |
  | `xrayStopped` | «Detenido» | el panel está en línea, pero Xray está detenido (`stop`) |

- Para este tipo de estado en la UI se usa **un indicador violeta separado** (color distinto del verde «en línea» y el rojo «fuera de línea»). El violeta indica directamente: se puede contactar el nodo, el problema está en el propio núcleo Xray y no en la red ni en el panel.

**Lo que ve el usuario:** en lugar del «verde» engañoso cuando el núcleo ha caído, el nodo se resalta en **violeta** con el estado **«Error de Xray»** o **«Detenido»**. Esto muestra inmediatamente que hay que reparar Xray en el nodo (reiniciar el núcleo, revisar `errorMsg`), y no investigar la accesibilidad del propio nodo. El mismo `xrayState`/`xrayError` también se transmite a los subnodos transitivos (véase el punto 1), por lo que el estado incorrecto del núcleo es visible en toda la cadena.

---

## 13. Configuración del panel

La sección «Configuración» (título de la página — **Configuración**, en inglés *Panel Settings*) controla el comportamiento del propio panel web 3X-UI: en qué dirección y puerto escucha, cómo está protegido, cómo interactúa con el bot de Telegram y los servicios externos, y en qué zona horaria ejecuta las tareas programadas. Cada parámetro se almacena en la tabla `settings` de la base de datos como un par «clave — valor»; si el valor no está en la BD, se aplica el valor por defecto.

> **Importante — aplicación de cambios.** Cualquier cambio en esta página debe guardarse con el botón **Guardar** (*Save*) y luego reiniciar el panel para que los cambios surtan efecto. El mensaje literal dice: «Guarda los cambios y reinicia el panel para aplicarlos.» Al guardar aparece la notificación «Configuración modificada».

### 13.1. Guardar y reiniciar el panel

| Elemento | Función |
| --- | --- |
| **Guardar** (*Save*) | Escribe todos los campos del formulario en la BD (`POST /panel/setting/update`). Antes de escribir, los valores pasan por validación — las direcciones, puertos o rutas incorrectas serán rechazados y el panel devolverá un error. |
| **Reiniciar panel** (*Restart Panel*) | Reinicia el servidor web del panel (`POST /panel/setting/restartPanel`). El reinicio ocurre con un retraso de 3 segundos. Mensaje literal: «¿Está seguro de que desea reiniciar el panel? Confirme y el reinicio ocurrirá en 3 segundos. Si el panel no está disponible, revise el log del servidor». Si tiene éxito: «El panel se reinició correctamente». |
| **Restaurar configuración predeterminada** (*Reset to Default*) | Elimina todos los ajustes guardados en la BD, tras lo cual el panel usa los valores por defecto. Las credenciales del administrador no se restablecen con esta operación. |

El reinicio se realiza enviando la señal `SIGHUP` al proceso del panel (o mediante el gancho de reinicio registrado). En Windows no se admite el reinicio automático mediante señal. **Los cambios en los parámetros de escucha (IP, puerto, ruta, dominio, certificados, zona horaria) solo se aplican tras reiniciar el panel.**

### 13.2. Configuración general (pestaña «Panel» / *General*)

#### Idioma de la interfaz (*Language*)

Idioma de la interfaz web del panel. Idiomas disponibles: `en-US` (inglés), `ru-RU` (ruso), `zh-CN`, `zh-TW`, `fa-IR`, `ar-EG`, `es-ES`, `id-ID`, `ja-JP`, `pt-BR`, `tr-TR`, `uk-UA`, `vi-VN`. Es una configuración de visualización y no afecta al funcionamiento de Xray.

#### Tipo de calendario (*Calendar Type*)

- **Clave:** `datepicker`
- **Valor por defecto:** `gregorian` (gregoriano).
- **Función:** tipo de calendario utilizado en los selectores de fecha (por ejemplo, al definir la fecha de vencimiento de los clientes). Mensaje literal: «Las tareas programadas se ejecutarán de acuerdo con este calendario.» El valor alternativo es el calendario persa (jalali), muy demandado entre el público iraní del panel.

#### Tamaño de paginación (*Pagination Size*)

- **Clave:** `pageSize`
- **Valor por defecto:** `25`
- **Valores admitidos:** entero de `0` a `1000`.
- **Función:** número de filas por página en las tablas (listas de conexiones/inbound). Mensaje literal: «Define el tamaño de página para la tabla de conexiones. Establece 0 para desactivar» — con `0` la paginación se desactiva y todos los registros se muestran en una sola lista.
- **No requiere reiniciar el panel** (ajuste de visualización).

#### Reiniciar Xray tras desactivación automática (*Restart Xray After Auto Disable*)

- **Clave:** `restartXrayOnClientDisable`
- **Valor por defecto:** `true`
- **Función:** cuando un cliente se desactiva automáticamente (por vencimiento del plazo o por alcanzar el límite de tráfico), Xray se reinicia para interrumpir las conexiones ya establecidas de ese cliente. Mensaje literal: «Cuando un cliente se desactiva automáticamente por vencimiento del plazo o límite de tráfico, reiniciar Xray.» La función en sí no ha cambiado — el interruptor simplemente se encuentra en la pestaña «Panel» (*General*) junto al resto de los ajustes generales.

#### Modelo de comentario y carácter separador (*Remark Model & Separation Character*)

- **Clave:** `remarkModel`
- **Valor por defecto:** `-ieo`
- **Función:** define cómo se forma el nombre (remark) de la configuración en la suscripción. La cadena consta del **primer carácter** — separador — seguido de la **secuencia de letras de orden**:
  - `i` — comentario del inbound (*inbound remark*);
  - `e` — email del cliente;
  - `o` — etiqueta adicional (*extra*).
  
  Con el valor por defecto `-ieo` el separador es `-` y el orden de las partes es: inbound → email → extra (por ejemplo, `MyInbound-user@mail-extra`). Las partes vacías se omiten. El campo «Ejemplo de comentario» (*Sample Remark*) en la interfaz muestra una vista previa del nombre generado. La inclusión del email en el nombre depende adicionalmente del parámetro «Incluir Email en el nombre» en la configuración de suscripción (sección sobre suscripciones).

**Ejemplo: cómo el valor de `remarkModel` afecta al nombre de la configuración.** Supongamos que el inbound se llama `VLESS-Reality`, el email del cliente es `alex@vpn` y la etiqueta adicional es `RU`. Entonces:

| Valor del campo | Nombre resultante (remark) |
| --- | --- |
| `-ieo` (por defecto) | `VLESS-Reality-alex@vpn-RU` |
| `_ie` | `VLESS-Reality_alex@vpn` |
| `-ei` | `alex@vpn-VLESS-Reality` |
| ` o` (espacio como separador, solo etiqueta) | `RU` |

El primer carácter de la cadena siempre es el separador; las demás letras determinan qué partes y en qué orden formarán el nombre.

### 13.3. Acceso al panel: IP, puerto, ruta, dominio, certificado

Este grupo define el punto de entrada de red del panel. **Todos los cambios aquí solo se aplican tras reiniciar el panel.**

| Campo | Clave | Valor por defecto | Descripción |
| --- | --- | --- | --- |
| Dirección IP de escucha del panel (*Listen IP*) | `webListen` | `""` (vacío) | IP en la que escucha el panel web. Vacío = escuchar en todas las IP. Mensaje literal: «Déjelo vacío para conectarse desde cualquier IP». Si se especifica, debe ser una dirección IP válida (de lo contrario el guardado se rechaza). |
| Dominio del panel (*Listen Domain*) | `webDomain` | `""` (vacío) | Nombre de dominio del panel para verificar la solicitud por dominio. Vacío = aceptar conexiones desde cualquier dominio e IP. Mensaje literal: «Déjelo vacío para conectarse desde cualquier dominio e IP.» |
| Puerto del panel (*Listen Port*) | `webPort` | `2053` | Puerto en el que funciona el panel. Mensaje literal: «Puerto en el que funciona el panel». Admite `1–65535`. El puerto debe estar libre; el panel y el servicio de suscripción no pueden usar simultáneamente el mismo par `IP:puerto`. |
| Ruta URI (*URI Path*) | `webBasePath` | `/` | Ruta base de la URL del panel (basePath). Mensaje literal: «Debe comenzar con '/' y terminar con '/'». Al guardar, el panel agrega automáticamente la barra inicial y final `/` si están ausentes. Se rechazan los caracteres no permitidos en la ruta. |

##### Certificado del panel (TLS / HTTPS)

| Campo | Clave | Valor por defecto | Descripción |
| --- | --- | --- | --- |
| Ruta al archivo de clave pública del certificado del panel (*Public Key Path*) | `webCertFile` | `""` | Ruta completa al archivo de certificado (cadena). Mensaje literal: «Introduzca la ruta completa comenzando con '/'». |
| Ruta al archivo de clave privada del certificado del panel (*Private Key Path*) | `webKeyFile` | `""` | Ruta completa al archivo de clave privada. Mensaje literal: «Introduzca la ruta completa comenzando con '/'». |

Si se especifica **al menos una** de las rutas de certificado/clave, al guardar el panel intenta cargar el par «certificado + clave»; si hay un error (archivo inexistente, clave y certificado no coinciden) el guardado se rechaza. Cuando ambas rutas correctas están definidas, el panel pasa a HTTPS. Ambos campos vacíos = el panel funciona con HTTP simple.

> **Advertencias de seguridad** (*Security warnings*). El panel muestra el bloque «Su panel puede estar expuesto:» con advertencias si detecta una configuración insegura:
> - funcionamiento con HTTP simple — «configure TLS para producción»;
> - puerto estándar 2053 — «cámbielo por uno aleatorio»;
> - ruta base por defecto `/` — «cámbiela por una aleatoria»;
> - ruta de suscripción estándar `/sub/` y de suscripción JSON `/json/` — «cámbielas».
> Son recomendaciones, no bloqueos.

### 13.4. Sesión, proxy del panel y proxies de confianza (pestaña «Proxy y servidor» / *Proxy and Server*)

#### Duración de sesión (*Session Duration*)

- **Clave:** `sessionMaxAge`
- **Valor por defecto:** `360` (minutos, es decir, 6 horas).
- **Valores admitidos:** de `1` a `525600` minutos (1 año).
- **Función:** cuánto tiempo permanece el administrador autenticado sin necesidad de volver a iniciar sesión. La unidad es el **minuto**. Mensaje literal: «Duración de la sesión en el sistema (valor: minuto)».

#### Outbound para el tráfico del panel (*Panel Traffic Outbound*)

- **Clave:** `panelOutbound`
- **Valor por defecto:** `""` (vacío = conexión directa).
- **Función:** define el **outbound** de Xray a través del cual el panel envía sus **propias solicitudes** — verificaciones de versiones y descarga del panel/Xray, comunicaciones con Telegram, actualización habitual de archivos geo — para eludir el filtrado del servidor de GitHub/Telegram. El campo es una **lista desplegable**: en ella se enumeran los outbounds de la plantilla de configuración de Xray, los outbounds de las suscripciones a outbound, así como los **balanceadores** de enrutamiento (en un grupo separado). Los outbounds de tipo `blackhole` están excluidos de la lista — no tiene sentido enrutar las descargas hacia un «agujero negro». Mensaje literal: «Enruta las solicitudes propias del panel — verificaciones de versiones y descargas del panel/Xray, Telegram y la actualización habitual de archivos geo — a través de este outbound de Xray para eludir el filtrado del servidor de GitHub/Telegram. Un inbound puente local se agrega automáticamente a la configuración en ejecución y se aplica al instante. La actualización automática de Geodata integrada en Xray no se ve afectada; tiene su propio outbound de descarga. Déjelo vacío para conexión directa.»

> **Cómo funciona.** Al seleccionar un outbound, el panel agrega por sí solo a la configuración activa un inbound loopback de servicio (puente SOCKS con la etiqueta `panel-egress`) y una regla de enrutamiento que redirige el propio tráfico HTTP del panel hacia el outbound seleccionado. Si se selecciona un balanceador, se inserta `balancerTag` en la regla y el tráfico del panel se distribuye entre sus miembros. El puente y la regla se aplican **al instante**, sin reiniciar completamente el panel. Deje el campo vacío para conexión directa. La actualización automática de datos geo integrada en Xray **no se ve afectada** por este ajuste — tiene su propio outbound dentro del enrutamiento de Xray.
- **Formato:** `socks5://` (o `socks5h://`) o `http(s)://`, con autenticación si es necesario, en la forma `socks5://user:pass@host:port`. Los esquemas admitidos son estrictamente: `socks5`, `socks5h`, `http`, `https` — otros esquemas se consideran no válidos y el panel revierte a conexión directa. Un ejemplo típico es el inbound SOCKS local del propio Xray.
- Mensaje literal: «Enruta las solicitudes salientes propias del panel (actualizaciones geo, verificaciones de versiones de Xray/panel, Telegram) a través de este proxy para eludir el filtrado del servidor de GitHub/Telegram. Acepta socks5:// o http(s)://, por ejemplo el inbound SOCKS local de Xray. Déjelo vacío para conexión directa.»
- Un proxy no válido no provoca un error al guardar — el panel simplemente utiliza la conexión directa y escribe una advertencia en el log.

**Ejemplo de valores del campo.** Si en el servidor ya hay un inbound SOCKS local de Xray en el puerto `10808`, dirija a través de él las solicitudes propias del panel:

```
socks5://127.0.0.1:10808
```

Para un proxy HTTP externo con autenticación:

```
http://user:pass@proxy.example.com:8080
```

Tras guardar y reiniciar, el panel obtendrá las actualizaciones de bases geo, verificará versiones y se comunicará con Telegram a través del proxy especificado.

#### CIDR de proxies de confianza (*Trusted proxy CIDRs*)

- **Clave:** `trustedProxyCIDRs`
- **Valor por defecto:** `127.0.0.1/32,::1/128` (solo el host local).
- **Formato:** lista de direcciones IP o subredes CIDR separadas por comas (por ejemplo `10.0.0.0/8, 192.168.1.5`). Cada elemento se verifica como IP o CIDR — un valor incorrecto se rechaza al guardar.
- **Función:** enumera las fuentes a las que se permite establecer los encabezados `X-Forwarded-Host`, `X-Forwarded-Proto` y el encabezado de IP real del cliente. Mensaje literal: «IP/CIDR separadas por comas a las que se permite establecer los encabezados forwarded host, proto e IP del cliente.» Es necesario configurarlo si el panel funciona detrás de un proxy inverso (nginx, Caddy, etc.) para determinar correctamente las IP de los clientes y el esquema.

**Ejemplo: panel detrás de un proxy inverso.** Si nginx está en el mismo host y reenvía solicitudes al panel, deje la confianza solo al host local (valor por defecto):

```
127.0.0.1/32,::1/128
```

Si el proxy se encuentra en un servidor separado dentro de la red interna `10.0.0.0/8`, agregue su subred; de lo contrario el panel ignorará los encabezados que este envíe y verá la IP del proxy en lugar de la del cliente real:

```
127.0.0.1/32,::1/128,10.0.0.0/8
```

Ejemplo del bloque nginx correspondiente que transmite la IP real y el esquema:

```nginx
proxy_set_header X-Real-IP        $remote_addr;
proxy_set_header X-Forwarded-For  $proxy_add_x_forwarded_for;
proxy_set_header X-Forwarded-Proto $scheme;
proxy_set_header X-Forwarded-Host $host;
```

### 13.5. Bot de Telegram (pestaña «Bot de Telegram» / *Telegram Bot*)

#### Habilitar bot de Telegram (*Enable Telegram Bot*)

- **Clave:** `tgBotEnable`
- **Tipo/por defecto:** booleano, `false`.
- **Función:** habilita el funcionamiento del bot de Telegram. Mensaje literal: «Acceso a las funciones del panel a través del bot de Telegram».

#### Token de Telegram (*Telegram Token*)

- **Clave:** `tgBotToken`
- **Por defecto:** `""`.
- **Función:** token del bot. Mensaje literal: «Debe obtener el token del gestor de bots de Telegram @botfather».
- **Particularidad de seguridad:** el token es un valor secreto. En la respuesta del panel a la lectura de configuración no se devuelve (el campo se vacía, solo se devuelve el indicador «configurado/no configurado»). Si al guardar se deja el campo vacío, el token guardado anteriormente **se conserva** (no se sobreescribe).

#### Idioma del bot de Telegram (*Telegram Bot Language*)

- **Clave:** `tgLang`
- **Por defecto:** `en-US`.
- **Función:** idioma de los mensajes del bot (independiente del idioma de la interfaz web). La lista de idiomas disponibles coincide con los idiomas del panel.

#### ID de usuario del administrador del bot (*Admin Chat ID*)

- **Clave:** `tgBotChatId`
- **Por defecto:** `""`.
- **Formato:** uno o varios User ID numéricos de Telegram **separados por comas**.
- **Función:** destinatarios de notificaciones y administradores a quienes se permite gestionar el panel a través del bot. Mensaje literal: «Uno o varios User ID del administrador/es del bot de Telegram. Para obtener el User ID use @userinfobot o el comando '/id' en el bot.»

#### Frecuencia de notificaciones (*Notification Time*)

- **Clave:** `tgRunTime`
- **Por defecto:** `@daily` (una vez al día).
- **Formato:** cadena en formato **Crontab** (se admiten tanto expresiones cron estándar como abreviaciones del tipo `@daily`, `@hourly`, `@every 1h`). Mensaje literal: «Especifique el intervalo de notificaciones en formato Crontab». Controla los informes periódicos del bot.

**Ejemplos de valores del campo.**

| Valor | Cuándo envía el bot el informe |
| --- | --- |
| `@daily` | una vez al día a medianoche (por defecto) |
| `@hourly` | cada hora |
| `@every 6h` | cada 6 horas |
| `0 9 * * *` | todos los días a las 09:00 |
| `30 8 * * 1` | cada lunes a las 08:30 |

La hora se calcula en la zona horaria del ajuste «Zona horaria» (punto 13.6).

#### Proxy SOCKS (*SOCKS Proxy*)

- **Clave:** `tgBotProxy`
- **Por defecto:** `""`.
- **Función:** proxy SOCKS5 específico para la conexión del bot a Telegram. Mensaje literal: «Si necesita un proxy Socks5 para conectarse a Telegram, configure sus parámetros según el manual.» Se aplica únicamente al tráfico del bot (distinto del «Proxy de red del panel» general del punto 13.4).

#### Servidor API de Telegram (*Telegram API Server*)

- **Clave:** `tgBotAPIServer`
- **Por defecto:** `""` (usar el servidor estándar `api.telegram.org`).
- **Formato:** URL `http(s)://…`; al guardar se verifica la validez de la URL — una dirección no válida se rechaza. Mensaje literal: «Servidor API de Telegram utilizado. Déjelo vacío para usar el servidor por defecto.» Es necesario para un servidor de la API del bot de Telegram desplegado de forma independiente.

#### Notificaciones del bot (grupo «Notificaciones» / *Notifications*)

| Campo | Clave | Por defecto | Descripción |
| --- | --- | --- | --- |
| Copia de seguridad de la base de datos (*Database Backup*) | `tgBotBackup` | `false` | Enviar a Telegram el archivo de copia de seguridad de la BD junto con el informe. Mensaje literal: «Enviar notificación con el archivo de copia de seguridad de la base de datos». |
| Notificación de inicio de sesión (*Login Notification*) | `tgBotLoginNotify` | `true` | Notificar ante intentos de inicio de sesión en el panel. Mensaje literal: «Muestra el nombre de usuario, la dirección IP y la hora cuando alguien intenta acceder a su panel.» |
| Antelación para notificación de vencimiento (*Expiration Date Notification*) | `expireDiff` | `0` | Cuántos **días** antes del vencimiento del cliente enviar la notificación. `0` — desactivado. Admite `>= 0`. Mensaje literal: «Recibir notificación de vencimiento de sesión antes de alcanzar el valor umbral (valor: día)». |
| Umbral de tráfico para notificación (*Traffic Cap Notification*) | `trafficDiff` | `0` | Umbral de tráfico restante para la notificación. Mensaje literal: «Recibir notificación de agotamiento de tráfico antes de alcanzar el umbral (valor: GB)». Admite `0–100`. |
| Umbral de carga de CPU (*CPU Load Notification*) | `tgCpu` | `80` | Notificar a los administradores si la carga de CPU supera el umbral (en **%**). Admite `0–100`. Mensaje literal: «Notificar a los administradores en Telegram si la carga de CPU supera este umbral (valor: %)». |

### 13.6. Fecha y hora (pestaña «Fecha y hora» / *Date and Time*)

#### Zona horaria (*Time Zone*)

- **Clave:** `timeLocation`
- **Valor por defecto:** `Local` (zona horaria del sistema del servidor).
- **Formato:** nombre de zona de la base de datos IANA tz (por ejemplo, `Europe/Moscow`, `UTC`, `Asia/Tehran`).
- **Función:** zona horaria en la que el panel ejecuta las tareas programadas (informes del bot, restablecimiento/verificaciones de tráfico, vencimientos de plazos). Mensaje literal: «Las tareas programadas se ejecutan de acuerdo con la hora en esta zona horaria».
- **Validación:** al guardar se verifica la zona — una zona inexistente se rechaza. Si posteriormente la BD contiene un valor incorrecto, el panel en tiempo de ejecución revierte a `Local` y, si este tampoco está disponible, a `UTC`.

### 13.7. Tráfico externo y comportamiento de Xray (pestaña «Tráfico externo» / *External Traffic*)

| Campo | Clave | Por defecto | Descripción |
| --- | --- | --- | --- |
| Información de tráfico externo (*External Traffic Inform*) | `externalTrafficInformEnable` | `false` | Notificar a la API externa en cada actualización de tráfico. Mensaje literal: «Notificar a la API externa en cada actualización de tráfico.» |
| URI de información de tráfico externo (*External Traffic Inform URI*) | `externalTrafficInformURI` | `""` | URL a la que el panel envía las actualizaciones de tráfico. Se verifica la validez de la URL al guardar. Mensaje literal: «Las actualizaciones de tráfico se envían a este URI». |
| Reiniciar Xray tras desactivación automática (*Restart Xray After Auto Disable*) | `restartXrayOnClientDisable` | `true` | Reiniciar Xray cuando un cliente se desactiva automáticamente por vencimiento del plazo o superación del límite de tráfico. Mensaje literal: «Cuando un cliente se desactiva automáticamente por vencimiento del plazo o límite de tráfico, reiniciar Xray.» **El interruptor se encuentra en la pestaña «Panel» (*General*)** — véase el punto 13.2; aquí se incluye a modo de referencia completa. |

### 13.8. Otros: plantilla de configuración de Xray y URL de verificación

#### Plantilla de configuración de Xray (*xrayTemplateConfig*)

- **Clave:** `xrayTemplateConfig`
- **Por defecto:** plantilla JSON integrada (embedded) incluida en la compilación.
- **Función:** plantilla JSON base de configuración de Xray-core sobre la cual el panel construye inbounds/outbounds. Este valor **no se devuelve** en la salida habitual de todos los ajustes y se edita en la página de configuración de Xray por separado, no en la lista general de campos de configuración del panel. La plantilla estándar por defecto está disponible mediante `GET /panel/setting/getDefaultJsonConfig`.

#### URL de verificación de outbounds (*xrayOutboundTestUrl*)

- **Clave:** `xrayOutboundTestUrl`
- **Por defecto:** `https://www.google.com/generate_204`
- **Función:** URL utilizada al verificar la operatividad de las conexiones salientes (outbound). Al establecerse pasa por saneamiento como URL HTTP(S).

### 13.9. Cuenta de administrador y tokens de API

Estos parámetros se encuentran en la pestaña adyacente («Cuenta» / *Authentication*) y se tratan en detalle en la sección de seguridad; aquí se incluye un resumen breve de las claves.

- **Cambio de credenciales** (campos «Nombre de usuario actual», «Contraseña actual», «Nuevo nombre de usuario», «Nueva contraseña») se guarda mediante una solicitud separada `POST /panel/setting/updateUser`. Se requieren el nombre de usuario y contraseña actuales correctos; el nuevo nombre de usuario y contraseña no deben estar vacíos. Mensajes: «Ha cambiado correctamente las credenciales del administrador.» / «Nombre de usuario o contraseña incorrectos».
- **Autenticación de dos factores (2FA)** — claves `twoFactorEnable` (por defecto `false`) y el secreto `twoFactorToken`. El token es secreto: con 2FA habilitado, dejar el campo vacío al guardar no sobreescribe el token existente. Al **habilitar** 2FA por primera vez, el panel invalida las sesiones actuales (se incrementa la «época de inicio de sesión»).
- **Los tokens de API** se gestionan mediante endpoints separados (`/panel/setting/apiTokens…`): lista, creación (`apiTokens/create`), eliminación, habilitación/deshabilitación. El propio token se muestra **solo una vez al crearlo** y no se almacena en formato legible: «Copie este token ahora. Por razones de seguridad no se almacena en formato legible y no volverá a mostrarse.»

Los detalles sobre 2FA, contraseñas, sincronización LDAP y formatos de suscripción (JSON/Clash, fragmentation, noises, mux) se tratan en las secciones correspondientes del manual.

### 13.10. Cambios de API en 3.3.0 (importante para integraciones)

En la versión 3.3.0 cambió la estructura de las rutas de la API del servidor. Si tiene integraciones externas (scripts, bots, paneles centrales, tareas CI) que acceden al panel por HTTP, **es necesario actualizarlas**; de lo contrario dejarán de funcionar.

#### ⚠️ BREAKING CHANGE: los endpoints `/panel/setting/*` y `/panel/xray/*` se han trasladado bajo `/panel/api`

Antes, la gestión de la configuración del panel y la configuración de Xray vivía por separado, bajo las rutas `/panel/setting/*` y `/panel/xray/*`. Ahora ambos conjuntos están registrados dentro del grupo de API común `/panel/api`. Las rutas antiguas **se han eliminado por completo** — una solicitud a ellas devolverá 404.

Por qué se hizo: todo el grupo `/panel/api` pasa por una verificación de acceso unificada, es decir, estos endpoints ahora aceptan el mismo encabezado `Authorization: Bearer <token>` que el resto de la API. El token de API es un acceso de administrador completo, y de esta forma toda la superficie de la API se ha vuelto uniforme.

**Lo que NO ha cambiado:** las páginas de la interfaz web (rutas SPA) `/panel/settings` y `/panel/xray` permanecen en su lugar — se habla únicamente de los endpoints de la API del servidor.

#### Tabla de correspondencia de rutas (antigua → nueva)

El prefijo de todas las rutas a continuación es — simplemente se añadió `api/` después de `/panel/`.

| Antes (≤ 3.2.x) | Ahora (3.3.0) | Método |
|---|---|---|
| `/panel/setting/all` | `/panel/api/setting/all` | POST |
| `/panel/setting/defaultSettings` | `/panel/api/setting/defaultSettings` | POST |
| `/panel/setting/update` | `/panel/api/setting/update` | POST |
| `/panel/setting/updateUser` | `/panel/api/setting/updateUser` | POST |
| `/panel/setting/restartPanel` | `/panel/api/setting/restartPanel` | POST |
| `/panel/setting/getDefaultJsonConfig` | `/panel/api/setting/getDefaultJsonConfig` | GET |
| `/panel/setting/apiTokens` | `/panel/api/setting/apiTokens` | GET |
| `/panel/setting/apiTokens/create` | `/panel/api/setting/apiTokens/create` | POST |
| `/panel/setting/apiTokens/delete/:id` | `/panel/api/setting/apiTokens/delete/:id` | POST |
| `/panel/setting/apiTokens/setEnabled/:id` | `/panel/api/setting/apiTokens/setEnabled/:id` | POST |
| `/panel/xray/` | `/panel/api/xray/` | POST |
| `/panel/xray/update` | `/panel/api/xray/update` | POST |
| `/panel/xray/getDefaultJsonConfig` | `/panel/api/xray/getDefaultJsonConfig` | GET |
| `/panel/xray/getXrayResult` | `/panel/api/xray/getXrayResult` | GET |
| `/panel/xray/getOutboundsTraffic` | `/panel/api/xray/getOutboundsTraffic` | GET |
| `/panel/xray/resetOutboundsTraffic` | `/panel/api/xray/resetOutboundsTraffic` | POST |
| `/panel/xray/testOutbound` | `/panel/api/xray/testOutbound` | POST |
| `/panel/xray/warp/:action` | `/panel/api/xray/warp/:action` | POST |
| `/panel/xray/nord/:action` | `/panel/api/xray/nord/:action` | POST |
| `/panel/xray/outbound-subs` (y `/outbound-subs/*`) | `/panel/api/xray/outbound-subs` (y `/outbound-subs/*`) | GET/POST/DELETE |

Los propios nombres de sub-rutas, los cuerpos de solicitudes y los formatos de respuesta no han cambiado — solo cambió el **prefijo**.

#### Cómo corregir las integraciones existentes

1. Busque en sus scripts/configuraciones todas las ocurrencias de `/panel/setting/` y `/panel/xray/`.
2. Reemplace el prefijo: añada `api/` inmediatamente después de `/panel/` (por ejemplo, `/panel/setting/all` → `/panel/api/setting/all`).
3. No es necesario modificar los cuerpos de solicitudes, los parámetros ni el formato de las respuestas — solo cambia la URL.
4. Como los ajustes y la configuración de Xray están ahora bajo `/panel/api`, se puede (y debe) acceder a ellos con el mismo token de API `Authorization: Bearer <token>` que a `/panel/api/inbounds/*` y demás endpoints. No olvide el middleware CSRF, que está habilitado para todo el grupo `/panel/api`.

**Ejemplo: lectura de todos los ajustes mediante la API.** Antes (≤ 3.2.x):

```bash
curl -sk -X POST "https://panel.example.com:2053/MyPath/panel/setting/all" \
  -H "Authorization: Bearer <token>"
```

Ahora (3.3.0) — se añadió `api/` después de `/panel/`:

```bash
curl -sk -X POST "https://panel.example.com:2053/MyPath/panel/api/setting/all" \
  -H "Authorization: Bearer <token>"
```

Del mismo modo para el reinicio del panel: `POST /panel/api/setting/restartPanel`. La ruta antigua `/panel/setting/restartPanel` ahora devolverá 404.

#### API tipada: esquemas y documentación (Swagger / OpenAPI)

En 3.3.0 la especificación OpenAPI pasó a ser totalmente tipada. Antes, las respuestas tipadas se describían con un objeto vacío `{}`; ahora los componentes y esquemas (`components.schemas`) se generan directamente a partir de los modelos de datos. Gracias a esto:

- La interfaz de Swagger UI muestra los modelos de datos reales en lugar de marcadores de posición genéricos.
- Los generadores externos (`openapi-generator` y similares) pueden construir clientes listos en el lenguaje deseado a partir de la especificación.
- Cada respuesta tipada lleva un `$ref` a un modelo concreto y ejemplos de respuestas adjuntos.

Dónde consultar la documentación de la API:

- **Página de Swagger integrada.** En el menú del panel — el elemento **«Documentación de API»** (ruta SPA `/panel/api-docs`). Aquí se enumeran de forma interactiva todos los endpoints con descripciones, cuerpos de solicitudes y ejemplos de respuestas.
- **La especificación OpenAPI 3.0 en bruto** se sirve en la dirección `/panel/api/openapi.json`. Esta URL puede pasarse directamente a Postman, Insomnia o `openapi-generator`. La especificación está integrada en el binario durante la compilación; cuando el panel se ejecuta bajo un `webBasePath` no estándar, el campo `servers` de la especificación se reescribe automáticamente según la ruta base actual, para que el botón «Try it out» y los generadores externos apunten al prefijo correcto.

---

## 14. Bot de Telegram

El panel 3X-UI incluye un bot de Telegram integrado que permite recibir notificaciones sobre el estado del servidor y los clientes, así como gestionar clientes individuales directamente desde el mensajero. El bot funciona mediante la tecnología long polling (consulta continua a Telegram), por lo que no requiere un dominio externo ni un puerto abierto: basta con tener acceso saliente a los servidores de Telegram.

El bot distingue dos tipos de interlocutores:

- **Administrador** — el usuario cuyo Telegram User ID está indicado en la configuración del bot (campo «User ID administrador del bot»). Tiene acceso a todas las funciones: estadísticas del servidor, copia de seguridad, gestión de clientes y reinicio de Xray.
- **Cliente** — cualquier otro usuario cuyo Telegram User ID esté vinculado a un cliente de inbound específico (campo `tgId` del cliente). Solo puede ver información sobre sus propias suscripciones.

**Ejemplo: vinculación de un cliente a Telegram.** Para que un usuario reciba estadísticas de su suscripción, su Telegram User ID numérico se registra en el campo `tgId` del cliente. En la configuración JSON del cliente tiene este aspecto:

```json
{
  "email": "ivan",
  "id": "6f1e6b1a-0c3d-4f2a-9b7e-1a2b3c4d5e6f",
  "tgId": "123456789",
  "enable": true,
  "limitIp": 2,
  "totalGB": 53687091200,
  "expiryTime": 0
}
```

Después de esto, el usuario con User ID `123456789` podrá solicitar al bot `/usage ivan` y ver sus estadísticas. El mismo ID puede asignarlo el administrador mediante el botón «👤 Establecer usuario de Telegram» en la tarjeta del cliente, sin necesidad de editar el JSON manualmente.

### 14.1. Activación y configuración del bot

Todos los parámetros del bot se configuran en el panel en la sección **Configuración → Telegram-бот**. Tras modificar la configuración, basta con guardarla: el panel la aplica de inmediato sin necesidad de reiniciarse. Si se cambia el indicador de activación (`tgBotEnable`), el token, los User ID de los administradores o la dirección del servidor API, el panel detiene automáticamente el bot y lo reinicia con los nuevos parámetros. La antigua regla que exigía reiniciar el panel tras cambiar el token ya no está vigente.

| Campo (UI) | Clave de configuración | Valor por defecto | Descripción |
|---|---|---|---|
| Включить Telegram бота | `tgBotEnable` | `false` | Interruptor principal. Descripción emergente: «Acceso a las funciones del panel mediante el bot de Telegram». Mientras esté desactivado, el bot no se inicia y las tareas de notificación no se programan. |
| Telegram-токен | `tgBotToken` | (vacío) | Token del bot. Descripción emergente: «Es necesario obtener el token del gestor de bots de Telegram @botfather». Sin un token válido, el bot no arranca. |
| SOCKS-прокси | `tgBotProxy` | (vacío) | Proxy para la conexión a Telegram. Descripción emergente: «Si necesita un proxy Socks5 para conectarse a Telegram, configure sus parámetros según el manual». |
| Telegram API Server | `tgBotAPIServer` | (vacío) | Servidor API alternativo de Telegram. Descripción emergente: «Servidor API de Telegram utilizado. Déjelo vacío para usar el servidor predeterminado». |
| User ID администратора бота | `tgBotChatId` | (vacío) | Uno o varios Telegram User ID de administradores separados por comas. Descripción emergente: «Para obtener el User ID use @userinfobot o el comando `/id` en el bot». |
| Частота уведомлений для администраторов от бота | `tgRunTime` | `@daily` | Programación del informe periódico en formato crontab. Descripción emergente: «Indique el intervalo de notificaciones en formato Crontab». |
| Резервное копирование базы данных | `tgBotBackup` | `false` | Descripción emergente: «Enviar notificación con el archivo de copia de seguridad de la base de datos». Adjunta la copia de seguridad al informe periódico. |
| Уведомление о входе | `tgBotLoginNotify` | `true` | Descripción emergente: «Muestra el nombre de usuario, la dirección IP y la hora cuando alguien intenta acceder a su panel». |
| Порог нагрузки на ЦП для уведомления | `tgCpu` | `80` | Umbral de carga de CPU en porcentaje (validación 0–100). Descripción emergente: «Notificar a los administradores en Telegram si la carga de CPU supera este umbral (valor: %)». Con valor 0, la comprobación de CPU queda desactivada. |
| Язык Telegram-бота | — | — | Idioma en el que el bot redacta todos los mensajes. |

#### Obtención del token mediante @BotFather

1. Abra en Telegram el diálogo con **@BotFather**.
2. Envíe el comando `/newbot` y siga las instrucciones (nombre del bot y `username` único que termine en `bot`).
3. BotFather proporcionará un token con el formato `123456789:AA...`. Cópielo en el campo **Telegram-токен**.

#### Obtención del User ID del administrador

El User ID es el identificador numérico de la cuenta (no el username). Puede obtenerse de dos formas:

- Escribir al bot **@userinfobot**.
- Iniciar el bot ya configurado y enviarle el comando **`/id`** — responderá con su ID.

Introduzca el número obtenido en el campo **User ID администратора бота**. Para designar varios administradores, enumere sus ID separados por comas (por ejemplo, `11111111,22222222`). Cada ID se valida como número entero; un valor incorrecto provocará un error al iniciar el bot.

**Ejemplo: valor del campo «User ID администратора бота».** Un único administrador — simplemente el número:

```
123456789
```

Dos administradores separados por coma (los espacios son opcionales):

```
123456789,987654321
```

Cada valor debe ser un número entero. Valores como `@username` o `123 456` (con espacio dentro del número) no son válidos: el bot no arrancará.

#### Proxy

Se admiten los esquemas `socks5://`, `http://` y `https://`. Si el campo del proxy está vacío, el bot intenta usar el proxy general del panel (si está configurado y su esquema es compatible). Las URL con esquema no admitido o sintaxis incorrecta se ignoran: el bot se conecta directamente. El proxy es útil cuando el acceso directo a la API de Telegram desde el servidor está bloqueado.

#### Notificaciones por correo electrónico (SMTP)

Además de Telegram, los mismos eventos pueden recibirse por correo. El canal se configura en la sección **Configuración → Email** en la pestaña **SMTP Settings**:

| Campo (UI) | Clave de configuración | Valor por defecto | Descripción |
|---|---|---|---|
| Enable Email Notifications | `smtpEnable` | `false` | Interruptor principal de las notificaciones por correo mediante SMTP. |
| SMTP Host | `smtpHost` | (vacío) | Host del servidor SMTP (por ejemplo, `smtp.gmail.com`). |
| SMTP Port | `smtpPort` | `587` | Puerto del servidor SMTP. |
| SMTP Username | `smtpUsername` | (vacío) | Nombre de usuario para la autenticación SMTP. Se utiliza también como dirección del remitente (From). |
| SMTP Password | `smtpPassword` | (vacío) | Contraseña para la autenticación SMTP. Se almacena de forma oculta; si la contraseña ya está configurada, el campo muestra el indicador «configurado» y puede dejarse vacío para conservar la actual. |
| Recipients | `smtpTo` | (vacío) | Lista de destinatarios separados por comas (por ejemplo, `admin@example.com, ops@example.com`). |
| Encryption | `smtpEncryptionType` | `starttls` | Tipo de cifrado de la conexión: `none` (sin cifrado), `starttls` (STARTTLS) o `tls` (TLS implícito). |

El botón **Send Test Email** envía un correo de prueba y muestra el resultado por etapas: **Connection** (conexión), **Authentication** (autenticación) y **Send** (envío). Si algo falla, el diagnóstico indica en qué etapa se produjo el error (por ejemplo, «Authentication failed — check username and password» o «Server requires STARTTLS — change encryption type»), lo que facilita el ajuste de los parámetros.

En la segunda pestaña (**Notifications**) se seleccionan los eventos sobre los que se recibirán correos, mediante las mismas tarjetas agrupadas que para Telegram (véase «Bus de eventos y selección de notificaciones» en la sección 14.5).

#### Servidor API de Telegram

De forma predeterminada, el bot se conecta a la API oficial de Telegram. En el campo **Telegram API Server** se puede indicar la dirección de un servidor Bot API propio (`telegram-bot-api`). La URL se verifica por razones de seguridad; una dirección bloqueada o incorrecta se descarta y se utiliza el servidor predeterminado.

### 14.2. Menú principal y botones

El menú se invoca con el comando **`/start`**. Los botones forman un teclado inline adjunto al mensaje; el conjunto de botones depende de si el usuario es administrador o cliente.

#### Menú del administrador

| Botón | Acción |
|---|---|
| 📊 Отсортированный отчёт об использовании трафика | Enumera todos los clientes ordenados por tráfico con el consumo de cada uno; los correos «sobrantes» sin datos se marcan con «❗ Нет результатов». |
| 💻 Состояние сервера | Resumen del servidor (véase la sección 14.5). El botón «🔄 Обновить» actualiza los datos. |
| Сбросить весь трафик | Restablece los contadores de tráfico de **todos** los clientes. Solicita confirmación («Вы уверены? 🤔»); después muestra «✅ Успешно» o «❌ Неудача» por cada cliente y, al finalizar, «🔚 Сброс трафика завершён для всех клиентов». |
| 📂 Бэкап БД | Envía el archivo de la base de datos y `config.json` (véase la sección 14.6). |
| 📄 Лог банов | Envía los archivos de registro de las direcciones IP bloqueadas por exceder el límite de IP. |
| 🔌 Входящие подключения | Resumen de todos los inbound: Remark, puerto, tráfico, número de clientes y fecha de vencimiento. |
| ⚠️ Скоро конец | Lista de inbound y clientes que pronto agotarán el tráfico o alcanzarán la fecha límite (véase la sección 14.5). |
| 🖱️ Команды | Muestra la ayuda de los comandos del administrador. |
| 🟢 Онлайн | Número y lista de clientes conectados en ese momento; al pulsar en un correo se abre la tarjeta del cliente. Botón «🔄 Обновить». |
| 👥 Все клиенты | Abre la selección de inbound y luego la lista de sus clientes para consultarlos o gestionarlos. |
| ➕ Новый клиент | Inicia el asistente de creación de clientes (selección de inbound → borrador → confirmación). |
| Настройки подписки / индивидуальные ссылки / QR-код | Selección de inbound y cliente para obtener el enlace de suscripción, los enlaces individuales o los códigos QR. |

#### Menú del cliente

El cliente dispone de un conjunto reducido de botones:

| Botón | Acción |
|---|---|
| Статистика клиента | Muestra los datos de todas las suscripciones vinculadas al Telegram User ID del cliente. |
| 🖱️ Команды | Muestra la ayuda de los comandos del cliente. |
| Настройки подписки | Selección del propio cliente → enlace de suscripción. |
| Индивидуальные ссылки | Selección del propio cliente → enlaces individuales. |
| QR-код | Selección del propio cliente → códigos QR. |

Si el usuario no tiene ningún cliente con su Telegram User ID, el bot responde: «❌ Ваша конфигурация не найдена! 💭 Пожалуйста, попросите администратора использовать ваш Telegram User ID в конфигурации. 🆔 Ваш User ID: …». Este ID debe comunicarse al administrador para que lo introduzca en el campo del cliente.

### 14.3. Comandos del bot

El bot tiene cuatro comandos registrados, visibles en el menú «/» de Telegram:

| Comando | Descripción (del menú) | Acceso | Qué hace |
|---|---|---|---|
| `/start` | Mostrar el menú principal | todos | Saludo; al administrador le muestra adicionalmente «🤖 Добро пожаловать в бота управления <Host>!» y el menú principal. |
| `/help` | Ayuda del bot | todos | Muestra un saludo general y la sugerencia de elegir una opción del menú. |
| `/status` | Comprobar el estado del bot | todos | Responde «✅ Бот функционирует нормально». |
| `/id` | Mostrar su Telegram ID | todos | Devuelve «🆔 Ваш User ID: <code>…</code>». Útil para conocer el propio User ID. |

Además de los comandos registrados, se procesan tres comandos con argumentos (no aparecen en el menú «/» pero funcionan):

- **`/usage [Email]`** — búsqueda de cliente por correo electrónico.
  - Para el **administrador** muestra la tarjeta completa del cliente (con botones de gestión).
  - Para el **cliente** muestra únicamente su propia suscripción con el correo indicado (según la vinculación del Telegram User ID). Sin argumento, el bot solicita indicar el correo: «❗ Пожалуйста, укажите email для поиска».
- **`/inbound [nombre de conexión]`** — solo para administrador. Busca el inbound por Remark y muestra sus parámetros con las estadísticas de todos los clientes. Sin argumento (o para un cliente) — «❗ Неизвестная команда».
- **`/restart`** — solo para administrador. Reinicia Xray Core. Respuestas posibles: «✅ Ядро Xray успешно перезапущено», «❗ Xray Core не запущен» (si el núcleo no está en ejecución), «❗ Ошибка при перезапуске Xray-core. <Ошибка>». Cualquier argumento tras `/restart` produce un mensaje de comando desconocido con la sugerencia `/restart`.

En los chats de grupo, un comando con formato `/comando@botusername` solo se procesa si el username coincide con el nombre del bot actual.

Ayuda del administrador (botón «Команды»):

```
🔃 Для перезапуска Xray Core: /restart
🔎 Для поиска клиента по email: /usage [Email]
📊 Для поиска входящих подключений (со статистикой клиентов): /inbound [имя подключения]
🆔 Ваш Telegram User ID: /id
```

Ayuda del cliente:

```
💲 Для просмотра информации о вашей подписке: /usage [Email]
🆔 Ваш Telegram User ID: /id
```

### 14.4. Gestión de clientes (solo administrador)

Al abrir la tarjeta de un cliente (mediante «Все клиенты», «Онлайн», «Скоро конец» o `/usage`), el administrador ve los datos del cliente (correo, inbound vinculados, estado «Activo», estado de conexión, fecha de vencimiento, consumo de tráfico) y los botones inline de gestión:

| Botón | Función |
|---|---|
| 🔄 Обновить | Volver a cargar la tarjeta del cliente. |
| 📈 Сбросить трафик | Restablecer el contador de tráfico del cliente. Requiere confirmación «✅ Подтвердить сброс трафика?». |
| 🚧 Лимит трафика | Establecer el límite de tráfico. Valores predefinidos: ♾ Sin límite (0), 1/5/10/20/30/40/50/60/80/100/150/200 GB o «🔢 Своё» — introducción de número mediante teclado numérico integrado (botones 0–9, «🔄» — reiniciar a 0, «⬅️» — borrar el último dígito, «✅ Подтвердить: N»). El valor se establece en gigabytes. |
| 📅 Изменить дату окончания | Opciones predefinidas: ♾ Sin límite, «🔢 Своё», añadir 7/10/14/20 días, 1/3/6/12 meses. Un número positivo prorroga el plazo (suma días a la fecha de vencimiento actual o a «ahora» si ya ha expirado); 0 elimina la restricción de plazo. |
| 🔢 Лог IP | Muestra las direcciones IP registradas del cliente (con marcas de tiempo si las hay). Desde el registro se puede «🔄 Обновить» y «❌ Очистить IP» (con confirmación «✅ Подтвердить очистку IP?»). |
| 🔢 Лимит IP | Límite de IP simultáneas. Opciones: ♾ Sin límite (0), 1–10 o «🔢 Своё» (teclado numérico). |
| 👤 Установить пользователя Telegram | Muestra el Telegram User ID vinculado actualmente al cliente; permite eliminar la vinculación («❌ Удалить пользователя Telegram» con confirmación). La vinculación de un nuevo usuario se realiza mediante el selector de contactos de Telegram del sistema. |
| 🔘 Вкл./Выкл. | Activa o desactiva al cliente. Requiere confirmación «✅ Подтвердить вкл/выкл пользователя?». |

Todas las operaciones que modifican la configuración (límite de tráfico/IP, fecha de vencimiento, vinculación/desvinculación de usuario de Telegram, activar/desactivar) marcan Xray para reinicio si es necesario, con el fin de que los cambios entren en vigor. Tras una operación exitosa, el bot muestra una confirmación del tipo «✅ <email>: …» y vuelve a mostrar la tarjeta.

Cualquier entrada numérica en los asistentes está limitada a valores < 999999.

### 14.5. Notificaciones e informes

Las notificaciones se envían a todos los administradores (a todos los User ID de `tgBotChatId`).

#### Bus de eventos y selección de notificaciones

Las notificaciones se basan en un bus de eventos único, y existen dos canales de entrega: **Telegram** y **correo electrónico (SMTP)**. Para cada canal se selecciona de forma independiente sobre qué eventos notificar. En **Configuración → Telegram** esto se hace en la pestaña **Notifications**; en **Configuración → Email**, en la pestaña homónima.

Los eventos están agrupados en tarjetas; cada grupo tiene un interruptor maestro con un contador de eventos activados (n/total) y un estado intermedio cuando solo una parte está seleccionada. Los grupos disponibles son:

- **Outbound** — «Down» (`outbound.down`) y «Up» (`outbound.up`): caída y recuperación del outbound.
- **Xray Core** — «Crash» (`xray.crash`): cierre inesperado del núcleo Xray.
- **Nodes** — «Down» (`node.down`) y «Up» (`node.up`): el nodo quedó inaccesible o se recuperó.
- **System** — «CPU high (%)» (`cpu.high`) y «Memory high (%)» (`memory.high`): alta carga del procesador y de la memoria RAM. Ambos eventos tienen junto a ellos un campo inline para el umbral en porcentaje.
- **Security** — «Login attempt» (`login.attempt`): intento de acceso al panel.

El conjunto de eventos activados se almacena por separado: para Telegram en `tgEnabledEvents`, para Email en `smtpEnabledEvents`. De forma predeterminada, en ambos canales están activados «Login attempt» y «CPU high» (valor `login.attempt,cpu.high`).

#### Notificación de acceso al panel

Controlada por la casilla **Уведомление о входе** (`tgBotLoginNotify`, activada por defecto). Ante cada intento de acceso a la interfaz web del panel, los administradores reciben un mensaje:

- En caso de éxito: «✅ Успешный вход в панель.» + host, nombre de usuario, IP y hora.
- En caso de fallo: «❗️ Ошибка входа в панель.» + host, **motivo** (por ejemplo, «Ошибка 2FA» si el segundo factor es incorrecto), nombre de usuario, IP y hora.

#### Superación del umbral de carga de CPU y memoria

El panel comprueba la carga del procesador y de la memoria RAM una vez por minuto. Si el umbral **`tgCpu`** > 0 y la carga media de CPU durante un minuto lo supera, los administradores reciben: «🔴 Загрузка процессора составляет N%, что превышает пороговое значение M%». De forma análoga se comprueba la carga de RAM frente al umbral **`tgMemory`** (80 % por defecto): evento «Memory high (%)».

Ambos umbrales se configuran mediante los campos inline situados junto a los eventos «CPU high (%)» y «Memory high (%)» en el grupo **System** de la pestaña Notifications (véase «Bus de eventos y selección de notificaciones» más arriba). Para el canal Email se utilizan las claves independientes `smtpCpu` y `smtpMemory`. Con el valor de umbral en 0, la comprobación correspondiente queda desactivada.

#### Informe periódico (programado)

Se programa mediante la expresión cron del campo **Частота уведомлений** (`tgRunTime`, valor por defecto `@daily`). Si el valor está vacío o es incorrecto, se utiliza `@daily`. El informe incluye:

#### Constructor de programación

El campo **Частота уведомлений для администраторов от бота** no se introduce manualmente como cadena de texto, sino mediante un constructor de programación. Primero se selecciona el modo en la lista desplegable:

- **`@every` — repetir con intervalo** — aparecen un campo numérico y la selección de unidad (**Секунды** / **Минуты** / **Часы**); el resultado se compone en una expresión del tipo `@every 6h`.
- **`@hourly` — cada hora**, **`@daily` — cada día a las 00:00**, **`@weekly` — cada semana**, **`@monthly` — cada mes** — preajustes listos que se guardan como el macro correspondiente (`@hourly`, `@daily`, `@weekly`, `@monthly`).
- **Произвольный (crontab)** — campo para una expresión crontab personalizada. El planificador del panel trabaja con segundos habilitados, por lo que la expresión personalizada consta de **6 campos**: segundo, minuto, hora, día del mes, mes, día de la semana (por ejemplo, `0 30 8 * * *` — cada día a las 08:30:00). Al cambiar a este modo, el campo se rellena con el equivalente crontab de la selección actual, como punto de partida.

**Ejemplo: valores del campo «Частота уведомлений» (`tgRunTime`).** Se admiten tanto abreviaciones predefinidas como el formato crontab completo:

| Valor | Cuándo se activa |
|---|---|
| `@daily` | una vez al día a medianoche (valor por defecto) |
| `@hourly` | cada hora |
| `@every 6h` | cada 6 horas |
| `0 9 * * *` | cada día a las 09:00 |
| `0 9 * * 1` | cada lunes a las 09:00 |
| `0 */12 * * *` | cada 12 horas (a las 00:00 y a las 12:00) |

Orden de los campos en crontab: minuto, hora, día del mes, mes, día de la semana.

1. La línea «🕰 Запланированные отчёты: <расписание>» y la fecha y hora actuales.
2. **Estado del servidor** (véase más abajo).
3. Bloque «Скоро конец» por inbound y clientes.
4. Notificaciones personales a los clientes con Telegram User ID vinculado — a cada cliente que no es administrador se le envía la lista de sus suscripciones que pronto agotarán el tráfico o el plazo (teniendo en cuenta las desactivadas).
5. Si está activado **Резервное копирование базы данных** (`tgBotBackup`) — copia de seguridad de la base de datos para los administradores.

**Estado del servidor** incluye: host, versión de 3X-UI y Xray, IPv4/IPv6, tiempo de actividad (en días), carga media (Load1/2/3), RAM (actual/total), número de clientes en línea, contadores de conexiones TCP/UDP, tráfico de red total (↑/↓) y estado de Xray.

**«Скоро конец»** muestra:

- por inbound: número de desactivados y número de los «que pronto se agotarán», seguido de la enumeración de dichos inbound (Remark, puerto, tráfico, fecha de vencimiento);
- por clientes: lo mismo, más tarjetas de clientes y botones con sus correos (al pulsarlos se abre la tarjeta del cliente).

Los umbrales de «próximo agotamiento» se toman de la configuración general del panel: margen de tráfico (en GB) y margen de plazo (en días). Se considera «próximo a agotarse» el inbound o cliente cuyo tráfico restante hasta el límite es inferior al umbral O cuyo tiempo restante hasta la fecha de vencimiento es inferior al umbral.

### 14.6. Copia de seguridad y registros

- **Copia de seguridad de la BD** (botón «📂 Бэкап БД» o casilla en el informe periódico): el bot envía la hora de la copia de seguridad, el archivo de la base de datos (`x-ui.db`, o `x-ui.dump` para PostgreSQL) y el archivo de configuración de Xray `config.json`.

El nombre del archivo de copia de seguridad enviado por el bot se forma a partir de la dirección del servidor: se utiliza el valor de **webDomain** o, si no está definido, la IP pública del servidor. Esto facilita identificar de qué servidor proviene el archivo cuando se reciben copias de seguridad de varios paneles. Si no se puede determinar la dirección, se usa un nombre genérico.
- **Registro de bloqueos** (botón «📄 Лог банов»): envía los archivos de registro actuales y anteriores de las direcciones IP bloqueadas por superar el límite de IP. Los archivos vacíos no se envían.

### 14.7. Particularidades de funcionamiento

- **Los mensajes largos** se dividen en partes (umbral ~2000 caracteres); el teclado inline se adjunta a la última parte.
- **Paralelismo**: los comandos y las pulsaciones de botones se procesan de forma concurrente (grupo de hasta 10 manejadores simultáneos).
- **Fiabilidad del envío**: ante errores de conexión, los mensajes se reenvían con retardo exponencial (1 s/2 s/4 s, hasta 3 intentos).
- **Caché**: los datos de «Estado del servidor» se cachean para que las pulsaciones frecuentes de «Обновить» no sobrecarguen el sistema.
- **Reinicio del bot**: al guardar la configuración que afecta al bot (indicador de activación, token, User ID de los administradores o dirección del servidor API), el panel detiene automáticamente el ciclo de consulta anterior e inicia uno nuevo con los parámetros actualizados, sin necesidad de recargar el panel. Solo se ejecuta una instancia de recepción de actualizaciones a la vez.

---

## 15. Bases geográficas (geoip / geosite y personalizadas)

Las bases geográficas son archivos binarios `.dat` que Xray-core utiliza para el enrutamiento y el filtrado de tráfico según la pertenencia a un país (rangos de IP) o según la categoría de dominios. El panel es capaz de descargar y actualizar tanto el conjunto estándar de archivos geográficos como fuentes personalizadas arbitrarias indicadas por URL. Todos los archivos se almacenan en el directorio `bin`, junto al binario de Xray (la ruta predeterminada es `bin`, y puede cambiarse mediante la variable de entorno `XUI_BIN_FOLDER`).

### 15.1. Qué son geoip.dat y geosite.dat

- **geoip.dat** — base de correspondencias «dirección IP → código de país/región». Se usa en reglas de enrutamiento con la forma `geoip:<código>`, por ejemplo `geoip:ru`, `geoip:cn`, así como para etiquetas especiales como `geoip:private` (redes privadas/locales). Conceptualmente, responde a la pregunta «¿en qué país se encuentra esta IP?».
- **geosite.dat** — base de correspondencias «dominio → categoría/lista». Se usa con la forma `geosite:<categoría>`, por ejemplo `geosite:category-ads-all` (dominios publicitarios), `geosite:google`, `geosite:ru`. Conceptualmente, son listas de dominios agrupadas por categoría.

Estos archivos son necesarios para construir reglas del tipo «todo el tráfico hacia IPs/dominios rusos va directo, el resto pasa por el outbound», y similares. Las propias reglas se configuran en la sección de enrutamiento de Xray; las bases geográficas únicamente les suministran los datos. Sin archivos geográficos actualizados, las reglas que hagan referencia a `geoip:`/`geosite:` no funcionarán o se basarán en listas desactualizadas.

**Ejemplo: regla «dominios e IPs rusos de forma directa».** Esta regla en la sección de enrutamiento dirige todo el tráfico hacia recursos rusos al outbound con la etiqueta `direct`:

```json
{
  "type": "field",
  "domain": ["geosite:category-ru"],
  "ip": ["geoip:ru"],
  "outboundTag": "direct"
}
```

### 15.2. Archivos geográficos estándar y su actualización

El panel contiene una lista de permisos (allowlist) fija de seis archivos estándar con fuentes de descarga predefinidas. La actualización se realiza a través de `POST /panel/api/server/updateGeofile/:fileName` (o sin nombre de archivo para actualizar todos a la vez).

**Ejemplo: actualización de un archivo y de todos a la vez a través de la API.** Actualizar solo `geoip_RU.dat`:

```bash
curl -X POST 'https://panel.example.com:2053/panel/api/server/updateGeofile/geoip_RU.dat' \
  -H 'Cookie: 3x-ui=<session-cookie>'
```

Actualizar los seis archivos estándar con una sola solicitud (sin especificar nombre de archivo):

```bash
curl -X POST 'https://panel.example.com:2053/panel/api/server/updateGeofile' \
  -H 'Cookie: 3x-ui=<session-cookie>'
```

Respuesta exitosa:

```json
{ "success": true, "msg": "Geofile updated successfully", "obj": null }
```

| Nombre de archivo | Fuente (repositorio de releases) |
|---|---|
| `geoip.dat` | `github.com/Loyalsoldier/v2ray-rules-dat` (geoip.dat) |
| `geosite.dat` | `github.com/Loyalsoldier/v2ray-rules-dat` (geosite.dat) |
| `geoip_IR.dat` | `github.com/chocolate4u/Iran-v2ray-rules` (geoip.dat) |
| `geosite_IR.dat` | `github.com/chocolate4u/Iran-v2ray-rules` (geosite.dat) |
| `geoip_RU.dat` | `github.com/runetfreedom/russia-v2ray-rules-dat` (geoip.dat) |
| `geosite_RU.dat` | `github.com/runetfreedom/russia-v2ray-rules-dat` (geosite.dat) |

Particularidades de la actualización de los archivos estándar:

- **Botón de actualización de un archivo.** Antes de la descarga se muestra una confirmación: «Do you really want to update the geofile? This will update the #filename# file.». Si la operación es exitosa, aparece la notificación «Geofile updated successfully».
- **Botón «Update all»** descarga los seis archivos. Confirmación: «This will update all geofiles.».
- **Descarga condicional.** Si el archivo ya existe localmente, se incluye en la solicitud el encabezado `If-Modified-Since` con la hora de modificación del archivo. Una respuesta `304 Not Modified` del servidor indica que el archivo no ha cambiado — no se vuelve a descargar, solo se actualiza la marca de tiempo del archivo.
- **Seguridad del nombre de archivo.** Solo se aceptan nombres que estén en la allowlist; el nombre se verifica para asegurarse de que no contiene `..`, separadores de ruta `/` ni `\`, rutas absolutas, y debe coincidir con el patrón `^[a-zA-Z0-9._-]+\.dat$`. Cualquier nombre fuera de la lista es rechazado con el error «Invalid geofile name».
- **Reinicio de Xray.** Tras la descarga de los archivos geográficos, Xray-core se reinicia para que relea las bases actualizadas. Si el reinicio falla, se añade la cadena de error correspondiente al mensaje de error.

#### Actualización de bases geográficas desde la línea de comandos (x-ui)

Las bases geográficas también pueden actualizarse sin el panel, a través del menú interactivo `x-ui` (opción de actualización de archivos geográficos) o mediante el comando no interactivo `x-ui update-all-geofiles`. Para cada archivo del conjunto (geoip/geosite, incluidos los conjuntos IR y RU) se muestra un estado individual: «actualizado», «ya está al día» o «error de descarga». En caso de descarga fallida, no se imprime ningún mensaje de éxito falso. El reinicio de Xray (y por tanto la interrupción de las conexiones activas) ocurre solo si al menos un archivo fue realmente actualizado; si ningún archivo cambió (todos devolvieron `304 Not Modified`), el panel y Xray no se reinician.

### 15.3. Actualización automática de geodatos mediante Xray (Geodata Auto-Update)

Las fuentes `.dat` adicionales con URL arbitrarias no se añaden a través del panel, sino mediante la sección nativa `geodata` de Xray-core. La sección correspondiente se encuentra en la ventana modal de actualizaciones de Xray (Dashboard → actualizaciones de Xray, `xrayUpdates`) — es la pestaña «Geodata Auto-Update». El panel aquí solo edita la clave `geodata` en la plantilla de configuración de Xray; la descarga, verificación y recarga en caliente de los archivos las gestiona el propio núcleo de Xray.

En la parte superior de la sección se muestra una sugerencia: «Xray downloads these files on schedule and hot-reloads them without a restart. URLs must be HTTPS. Each file must already exist in the bin folder once before Xray can update it.».

#### Campos de la sección

- **Schedule (cron)** — cadena cron de 5 campos; el valor predeterminado es `0 4 * * *` (diariamente a las 04:00). Al guardar, se verifica que la cadena contenga exactamente 5 campos; de lo contrario, se muestra el error «Cron debe contener 5 campos, p. ej. 0 4 * * *».
- **Download through outbound (optional)** — lista desplegable con las etiquetas de los outbound disponibles (más los outbound de suscripciones) a través del cual Xray descargará los archivos; los outbound con protocolo `blackhole` no aparecen en la lista. El campo puede dejarse vacío — en ese caso se usará una conexión directa. Esta elección es independiente del outbound para las propias solicitudes del panel (véase §11): la actualización automática de geodata tiene su propio outbound de descarga separado.
- **Lista de archivos** — cada fila define un par «URL + Nombre de archivo» (*File name*). La URL debe comenzar con `https://` (de lo contrario: «Se requiere una URL HTTPS para cada archivo.»). El nombre de archivo se indica de forma simple, sin rutas ni separadores — solo los caracteres `^[A-Za-z0-9._-]+$` (de lo contrario: «El nombre de archivo debe ser simple, por ejemplo geosite_custom.dat (sin rutas).»). Al introducir la URL, el panel intenta completar automáticamente el nombre de archivo a partir del último segmento de la ruta. El botón «Add file» añade una fila; el botón de papelera la elimina.

Si la lista está vacía, se muestra la sugerencia: «No files configured. Reference files in routing rules as ext:geosite_custom.dat:category.».

#### Guardado

El botón «Save & Restart Xray» muestra la confirmación «Save geodata settings? This updates the Xray config template and restarts Xray.». Tras guardar, la clave `geodata` se escribe en la plantilla de configuración (`POST /panel/api/xray/update`) y Xray se reinicia (`POST /panel/api/server/restartXrayService`). Si la lista de archivos está vacía, la clave `geodata` se elimina de la plantilla.

Aspectos importantes:

- **El archivo debe existir previamente en `bin`.** Xray solo actualiza los archivos `.dat` que ya están presentes en la carpeta `bin` en el momento del arranque. Por eso, un nuevo archivo personalizado primero debe colocarse manualmente en `bin` (o al menos crear allí una versión vacía/desactualizada con el nombre requerido), y solo entonces Xray empezará a mantenerlo actualizado según el calendario.
- **Recarga en caliente.** Tras la descarga programada, Xray relee las bases actualizadas sin reiniciar el proceso por completo.
- **Compatibilidad.** Los archivos geográficos descargados anteriormente (tanto los estándar como los personalizados) siguen funcionando en las reglas de enrutamiento con la sintaxis `ext:` sin cambios.

Si la lista está vacía, se muestra la sugerencia: «No custom geo sources yet — click Add to create one».

#### Columnas de la tabla y campos de la fuente

| Campo (UI) | JSON | Valor predeterminado | Descripción |
|---|---|---|---|
| Tipo (*Type*) | `type` | — (obligatorio) | Tipo de recurso: solo `geosite` o `geoip`. Determina el nombre del archivo resultante. |
| Alias (*Alias*) | `alias` | — (obligatorio) | Identificador corto de la fuente. A partir de él y del tipo se construye el nombre del archivo. |
| URL (*URL*) | `url` | — (obligatorio) | Enlace directo al archivo `.dat` (http/https). |
| Habilitado (*Enabled*) | — | — | Indicador de actividad de la fuente en la lista. |
| Actualizado (*Last updated*) | `lastUpdatedAt` | `0` | Hora de la última actualización exitosa (tiempo Unix; `0` — todavía no se ha actualizado). |
| Enrutamiento (ext:…) (*Routing (ext:…)*) | — | — | Cadena lista para usar en reglas de enrutamiento: `ext:<archivo.dat>:tag`. |
| Acciones (*Actions*) | — | — | Botones «Editar», «Eliminar», «Actualizar ahora». |

Adicionalmente, en la base de datos se almacenan campos internos: `localPath` (ruta real al archivo en el directorio `bin`), `lastModified` (valor del encabezado `Last-Modified` del servidor, usado para la descarga condicional), `createdAt` y `updatedAt`.

#### Nomenclatura de archivos

El nombre del archivo resultante se genera automáticamente a partir del tipo y el alias:

- tipo `geoip` → `geoip_<alias>.dat`;
- tipo `geosite` → `geosite_<alias>.dat`.

Por ejemplo, una fuente con tipo `geosite` y alias `myads` creará el archivo `geosite_myads.dat`.

**Ejemplo: añadir una fuente a través de la API.** Añadir una lista propia de dominios publicitarios como recurso `geosite` con el alias `myads`:

```bash
curl -X POST 'https://panel.example.com:2053/panel/api/server/customGeo/add' \
  -H 'Cookie: 3x-ui=<session-cookie>' \
  -H 'Content-Type: application/json' \
  -d '{
    "type": "geosite",
    "alias": "myads",
    "url": "https://example.com/lists/myads.dat"
  }'
```

El panel descargará el archivo en el directorio `bin` como `geosite_myads.dat`, guardará el registro y reiniciará Xray.

#### Botones y acciones

- **Añadir** (*Add*) — abre el formulario «Add custom geo». El botón de guardado es «Save». API: `POST /add`.
- **Editar** (*Edit*) — formulario «Edit custom geo». API: `POST /update/:id`. Al cambiar el tipo o el alias, el archivo antiguo se elimina y el nuevo se descarga de nuevo.
- **Eliminar** (*Delete*) — confirmación «Delete this custom geo source?». Elimina el registro de la base de datos y el propio archivo `.dat`. API: `POST /delete/:id`. Si la operación es exitosa: «Pользовательский geo-файл «<nombre>» eliminado».
- **Actualizar ahora** (*Update now*) — vuelve a descargar la fuente concreta y actualiza la marca de tiempo. API: `POST /download/:id`. Si la operación es exitosa: «Geofile «<nombre>» actualizado».
- **Actualizar todo** — actualiza todas las fuentes personalizadas a la vez. API: `POST /update-all`. Si todo es exitoso: «All custom geo sources updated». Si al menos una fuente no se actualizó, la operación se considera parcialmente fallida con el mensaje «One or more custom geo sources failed to update», y en la respuesta se enumeran las fuentes exitosas y las fallidas.

Tras cualquiera de las acciones (añadir, editar, eliminar, actualizar, actualizar todo cuando hay éxitos), Xray-core se reinicia.

#### Paso a paso: añadir una fuente

1. Pulse «Añadir».
2. En el campo «Tipo» seleccione `geosite` o `geoip`.
3. En el campo «Alias» introduzca el identificador (solo letras latinas minúsculas, dígitos, `-` y `_`; texto de marcador de posición: `a-z 0-9 _ -`).
4. En el campo «URL» indique el enlace directo al archivo `.dat` (debe comenzar con `http://` o `https://`).
5. Pulse «Guardar». El panel descargará inmediatamente el archivo en el directorio `bin`, guardará el registro y reiniciará Xray.

### 15.4. Validación y restricciones

Al crear y modificar una fuente se realizan verificaciones estrictas. Mensajes de error:

| Condición | Mensaje (RU) | Mensaje (EN) |
|---|---|---|
| Tipo distinto de `geosite`/`geoip` | Тип должен быть geosite или geoip | *Type must be geosite or geoip* |
| Alias vacío | Укажите псевдоним | *Alias is required* |
| Caracteres no permitidos en el alias (no coincide con `^[a-z0-9_-]+$`) | Псевдоним содержит недопустимые символы | *Alias must match allowed characters* |
| Alias reservado | Этот псевдоним зарезервирован | *This alias is reserved* |
| URL vacía | Укажите URL | *URL is required* |
| URL no analizable | Некорректный URL | *URL is invalid* |
| Esquema distinto de http/https | URL должен использовать http или https | *URL must use http or https* |
| Host vacío/incorrecto o bloqueado por la protección SSRF | Некорректный хост URL | *URL host is invalid* |
| Duplicado «tipo + alias» | Такой псевдоним уже используется для этого типа | *This alias is already used for this type* |
| Fuente no encontrada | Источник не найден | *Custom geo source not found* |
| Error de descarga | Ошибка загрузки | *Download failed* |

Sugerencias en el formulario (validación en el cliente): «Alias may only contain lowercase letters, digits, - and _» y «URL must start with http:// or https://».

Restricciones técnicas adicionales:

- **Aliases reservados.** No se pueden usar aliases que entren en conflicto con los archivos estándar. Están reservados (la comparación es insensible a mayúsculas y minúsculas, y el guion se equipara al guion bajo): `geoip`, `geosite`, `geoip_ir`, `geosite_ir`, `geoip_ru`, `geosite_ru`. Por ejemplo, `geosite-ru` será rechazado como `geosite_ru`.
- **Protección SSRF.** El host de la URL se resuelve a IP, y si apunta a una dirección privada/interna, la descarga queda bloqueada (el usuario ve «URL host is invalid»). Esto impide usar el panel para acceder a servicios internos.
- **Protección contra path traversal.** La ruta final del archivo debe encontrarse dentro del directorio `bin` (con resolución de enlaces simbólicos); cualquier intento de salir de ese directorio es rechazado.
- **Tamaño mínimo del archivo.** Un archivo descargado se considera válido solo si tiene al menos 64 bytes; un archivo demasiado pequeño se rechaza con un error de descarga.
- **Proxy y descarga condicional.** Si en la configuración del panel se ha definido un proxy, la descarga se realiza a través de él; en caso contrario, se utiliza una conexión directa con transporte seguro frente a SSRF. Al igual que con los archivos estándar, se aplica `If-Modified-Since`/`304 Not Modified` (un archivo sin cambios no se vuelve a descargar). El tiempo de espera de descarga es de 10 minutos; la comprobación de accesibilidad de la URL (HEAD y, si es necesario, GET parcial) es de 12 segundos.

### 15.5. Verificación automática al arrancar el panel

Al iniciarse, el panel recorre todas las fuentes personalizadas y verifica la existencia e integridad de cada archivo local (el archivo no existe, es un directorio o tiene menos de 64 bytes). Si el archivo falta o está dañado, se realiza una comprobación de la fuente y un intento de nueva descarga. Esto garantiza que, tras una reinstalación o pérdida del directorio `bin`, los archivos geográficos personalizados se restauren automáticamente.

### 15.6. Uso de las bases geográficas en las reglas de enrutamiento

En las reglas de enrutamiento de Xray, las bases geográficas se usan en campos como `domain`/`ip` mediante prefijos:

- **geoip:** para bases de IPs — `geoip:<código>`. Ejemplos: `geoip:ru`, `geoip:cn`, `geoip:private`. Se obtiene de `geoip.dat` (o de `geoip_RU.dat`, etc., si la regla apunta a un archivo específico).
- **geosite:** para bases de dominios — `geosite:<categoría>`. Ejemplos: `geosite:category-ads-all`, `geosite:google`, `geosite:ru`. Se obtiene de `geosite.dat`.

**Ejemplo: bloqueo de publicidad con geosite.** Regla que envía todos los dominios publicitarios a un «agujero negro» (se presupone un outbound con la etiqueta `blocked` y el protocolo `blackhole`):

```json
{
  "type": "field",
  "domain": ["geosite:category-ads-all"],
  "outboundTag": "blocked"
}
```

Para archivos **personalizados** se usa la sintaxis de archivo externo `ext:`. La sugerencia en la interfaz dice: «In routing rules use the value column as ext:file.dat:tag (replace tag).». Formato:

```
ext:<nombre_archivo.dat>:<etiqueta>
```

donde `<nombre_archivo.dat>` es `geoip_<alias>.dat` o `geosite_<alias>.dat`, y `<etiqueta>` es la lista/categoría concreta dentro del archivo. El panel, en la columna «Enrutamiento (ext:…)», sugiere una plantilla lista del tipo `ext:geosite_myads.dat:tag` — solo hay que reemplazar `tag` por la etiqueta deseada. El nombre de dicho archivo se define en la sección «Geodata Auto-Update» (véase §15.3) en el campo «Nombre de archivo» — por ejemplo `geosite_custom.dat`; en las reglas se hace referencia a él como `ext:geosite_custom.dat:category`.

**Ejemplo: regla basada en un archivo personalizado.** Si se ha añadido una fuente de tipo `geosite` con el alias `myads`, y dentro del archivo `.dat` la lista está marcada con la etiqueta `ads`, la regla de enrutamiento es la siguiente:

```json
{
  "type": "field",
  "domain": ["ext:geosite_myads.dat:ads"],
  "outboundTag": "blocked"
}
```

Para una fuente de IPs (tipo `geoip`, alias `mycorp`, etiqueta `office`), el campo sería `"ip": ["ext:geoip_mycorp.dat:office"]`.

---

## 16. Operaciones: copias de seguridad, registros, actualización, CLI

Esta sección cubre el mantenimiento diario del panel: creación y restauración de copias de seguridad de la base de datos, visualización de registros (logs) del panel y de Xray, reinicio y detención de servicios, actualización de Xray y del propio panel, tareas periódicas (cron) y eliminación del panel. Algunas operaciones se realizan desde la interfaz web (pestañas en las páginas «Dashboard» y «Configuración del panel»), otras desde el menú de consola `x-ui` en el servidor.

### 16.1. Copia de seguridad y restauración de la base de datos

Todos los datos del panel (inbound, clientes, grupos, nodos, configuración) se almacenan en una sola base de datos. La gestión de copias de seguridad está disponible en la página **«Dashboard»** en la pestaña **«Copia de seguridad»**, con el encabezado del bloque **«Copia de seguridad y restauración»**.

El panel admite dos motores de base de datos, y el comportamiento de la copia de seguridad depende de ello:

- **SQLite** (opción predeterminada) — los datos se almacenan en el archivo `x-ui.db`.
- **PostgreSQL** — si el panel está configurado con PostgreSQL, el bloque muestra el siguiente aviso:
  > «Este panel funciona con PostgreSQL. «Copia de seguridad» descarga un archivo pg_dump (.dump) y «Restaurar» lo carga de nuevo mediante pg_restore. Las herramientas cliente de PostgreSQL (pg_dump y pg_restore) deben estar instaladas en el servidor.»

#### Exportar (crear copia)

El botón **«Exportar base de datos»** (`Back Up`) descarga el archivo de copia de seguridad en su dispositivo.

| Motor de BD | Nombre del archivo | Qué ocurre en el servidor |
|-------------|-------------------|--------------------------|
| SQLite | `x-ui.db` | Primero se realiza un checkpoint WAL para que el archivo contenga los registros más recientes; luego el archivo se lee íntegramente y se envía para descargar |
| PostgreSQL | `x-ui.dump` | Se ejecuta `pg_dump` y el archivo se envía para descargar |

Mensajes de ayuda en la interfaz:
- SQLite: «Haga clic para descargar el archivo .db que contiene una copia de seguridad de su base de datos actual a su dispositivo.»
- PostgreSQL: «Haga clic para descargar un volcado de PostgreSQL (.dump) de la base de datos actual a su dispositivo.»

Técnicamente, la exportación es una solicitud `GET /panel/api/server/getDb`. El nombre del adjunto lo genera el servidor (`Content-Disposition`) según el motor utilizado.

El nombre del archivo de copia de seguridad se forma a partir de la dirección del servidor, no con los valores fijos `x-ui.db` / `x-ui.dump`. Al descargarlo desde el navegador, se obtiene de la dirección del panel en la barra de direcciones (host de la solicitud); de lo contrario, del dominio web configurado; y si no existe, de la IP pública del servidor (primero IPv4, luego IPv6), con retroceso a `x-ui`. Esto facilita distinguir las copias de seguridad de distintos servidores. La extensión sigue siendo `.db` para SQLite y `.dump` para PostgreSQL; las copias de seguridad enviadas por Telegram se nombran según el mismo dominio/IP.

**Ejemplo: descarga de la copia de seguridad mediante la API.** La misma exportación puede obtenerse con una solicitud desde la consola, por ejemplo para un script de copia de seguridad automática. Se necesita una sesión autenticada (cookie de inicio de sesión):

```bash
# 1) Iniciamos sesión y guardamos la cookie de sesión
curl -s -c cookies.txt \
     -d 'username=admin&password=admin' \
     https://panel.example.com:2053/panel/login

# 2) Descargamos el archivo de base de datos (el nombre lo establece el servidor: x-ui.db o x-ui.dump)
curl -s -b cookies.txt -OJ \
     https://panel.example.com:2053/panel/api/server/getDb
```

Si el panel está abierto bajo una ruta base (Web Base Path), debe añadirse al URL: `…:2053/<base_path>/panel/api/server/getDb`.

#### Importar (restaurar)

El botón **«Importar base de datos»** (`Restore`) abre el selector de archivos y sube el archivo al servidor para restaurarlo (`POST /panel/api/server/importDB`, campo de formulario `db`).

Mensajes de ayuda en la interfaz:
- SQLite: «Haga clic para seleccionar y subir un archivo .db desde su dispositivo para restaurar la base de datos desde la copia de seguridad.»
- PostgreSQL: «Haga clic para seleccionar y subir un archivo .dump para restaurar la base de datos de PostgreSQL. Esto reemplazará todos los datos actuales.»

**Proceso de importación para SQLite (importante entender que es atómico y con reversión):**
1. El archivo subido se valida como formato correcto — debe ser una base SQLite válida; de lo contrario, se devuelve el error «Invalid db file format».
2. El archivo se guarda como `x-ui.db.temp` temporal y se verifica su integridad.
3. **Xray se detiene** antes de reemplazar la BD.
4. La base de datos actual se renombra como `x-ui.db.backup` (respaldo de emergencia).
5. El archivo temporal se mueve a la ubicación de la BD activa, se realiza la inicialización y las migraciones de esquema, y luego la migración de inbound.
6. **Si cualquier paso falla** — se realiza una reversión: se restaura la base anterior desde `x-ui.db.backup`, y Xray se reinicia con los datos anteriores.
7. Si todo tiene éxito, el archivo de respaldo se elimina y **Xray se reinicia automáticamente** con los datos restaurados.

Mensajes de la interfaz según el resultado:

| Resultado | Texto |
|-----------|-------|
| Éxito | «Base de datos importada correctamente» |
| Error de importación | «Se produjo un error al importar la base de datos» |
| Error de lectura del archivo | «Se produjo un error al leer la base de datos» |

> La restauración reemplaza completamente los datos actuales. Dado que Xray se detiene brevemente durante el proceso, las conexiones activas de los clientes se interrumpen durante la importación.

#### Archivo de migración entre motores (SQLite ⇄ PostgreSQL)

Aparte de la copia de seguridad habitual, existe la función **«Descargar archivo de migración»** (`Download Migration`, solicitud `GET /panel/api/server/getMigration`). Genera un archivo portable para cambiar de motor de BD:

| Motor actual | Qué se descarga | Nombre del archivo | Propósito |
|---|---|---|---|
| SQLite | Volcado SQL portable (texto) | `x-ui.dump` | Inicializar PostgreSQL con sus datos |
| PostgreSQL | Base SQLite construida a partir de los datos de PostgreSQL | `x-ui.db` | Volver el panel a SQLite |

Mensajes de ayuda:
- En SQLite: «Haga clic para descargar una exportación portable .dump (texto SQL) de su base de datos SQLite.»
- En PostgreSQL: «Haga clic para descargar una base de datos SQLite (.db) construida a partir de sus datos de PostgreSQL y lista para ejecutar el panel en SQLite.»

La conversión `.db ⇄ .dump` para SQLite también puede realizarse desde la CLI con el comando `x-ui migrateDB [file]` (véase la sección 16.7).

#### Copia de seguridad mediante el bot de Telegram

Si el bot de Telegram está configurado (véase la sección sobre notificaciones), puede enviar una copia de seguridad directamente al chat del administrador. La copia de seguridad por Telegram incluye **dos archivos**: la propia base de datos (`x-ui.db`, o `x-ui.dump` en PostgreSQL) y la configuración de Xray `config.json`. El mensaje va precedido de la línea «🗄 Tiempo de copia de seguridad: …».

Hay dos formas de obtener la copia de seguridad en Telegram:

1. **A petición.** El botón **«📂 Copia de seguridad de BD»** en el menú del bot hace que este envíe inmediatamente los archivos al chat actual.
2. **Automáticamente con el informe.** En la configuración del bot hay un interruptor **«Copia de seguridad de base de datos»** (`Database Backup`) con la descripción «Enviar notificación con el archivo de copia de seguridad de la base de datos». Cuando está activado, con cada envío periódico del informe, el bot adjunta la copia de seguridad a todos los administradores después del informe. El período de envío del informe se establece mediante la programación cron del bot (véase la sección 16.6). Entre archivos y entre administradores, el bot hace pausas para no exceder los límites de Telegram.

> La copia de seguridad mediante el bot se envía solo si el bot está en ejecución; en PostgreSQL también requiere que `pg_dump` esté instalado en el servidor.

### 16.2. Visualización de registros

El panel cuenta con dos visores de registros independientes, ambos accesibles desde la pestaña **«Registros»** en el «Dashboard». Cada ventana puede actualizarse (icono «actualizar» en el encabezado) y permite descargar el contenido mostrado en un archivo `x-ui.log` (botón con icono de descarga).

#### Registros del panel (aplicación / syslog)

Ventana de registros del panel (`POST /panel/api/server/logs/{count}`). Controles:

| Elemento | Valor predeterminado | Descripción |
|----------|---------------------|-------------|
| Número de líneas | `20` | Lista desplegable: 20 / 50 / 100 / 500 / 1000 |
| Nivel | `Info` | Nivel mínimo: Debug / Info / Notice / Warning / Error |
| SysLog (casilla) | desactivado | Origen de los registros: del buffer interno de la aplicación o del diario del sistema |
| **Actualización automática** (casilla) | desactivado | Volver a leer el registro cada 5 segundos (véase más abajo) |

El comportamiento depende de la casilla **SysLog**:

- **Desactivado (predeterminado):** los registros se obtienen del buffer circular interno del panel, filtrados por el nivel seleccionado. Las entradas se muestran con nivel (DEBUG / INFO / NOTICE / WARNING / ERROR) y origen: `X-UI:` — mensajes del propio panel, `XRAY:` — mensajes redirigidos de Xray.

> Los mensajes simples sin marca de tiempo ni nivel (por ejemplo, el mensaje del sistema «Syslog is not supported» en Windows) ahora se muestran íntegramente tal como son. Solo se reconoce estrictamente el formato `YYYY/MM/DD LEVEL - cuerpo`; todo lo demás se muestra sin análisis, por lo que estas líneas ya no se truncan (antes las primeras tres palabras se interpretaban erróneamente como fecha/hora/nivel).
- **Activado:** el panel ejecuta en el servidor `journalctl -u x-ui --no-pager -n <count> -p <level>`, es decir, muestra el diario del sistema del servicio `x-ui`. El número de líneas permitido va de 1 a 10000; el nivel acepta valores syslog (`emerg/0`, `alert/1`, `crit/2`, `err/3`, `warning/4`, `notice/5`, `info/6`, `debug/7`). En Windows el modo SysLog no está soportado — se mostrará una advertencia indicando que hay que desmarcar la casilla y usar los registros de la aplicación. Si `systemd`/el servicio no están disponibles, aparecerá un mensaje de error al iniciar `journalctl`.

**Ejemplo: el mismo diario desde la consola del servidor.** Cuando el panel no está disponible (por ejemplo, no arranca), el diario del sistema puede leerse directamente — es exactamente el comando que el panel ejecuta en modo SysLog:

```bash
# últimas 100 líneas de nivel warning y superior
journalctl -u x-ui --no-pager -n 100 -p warning

# seguir el diario en tiempo real
journalctl -u x-ui -f
```

> El nivel en esta ventana filtra la **salida**. El nivel mínimo que se escribe en la consola/syslog lo determina el nivel de registro del panel (variable de entorno, predeterminado `Info`; en el archivo, el panel siempre escribe al nivel `DEBUG`).

#### Registros de acceso de Xray (diario de acceso)

Ventana separada para el registro de acceso de Xray (`POST /panel/api/server/xraylogs/{count}`). Analiza las líneas del diario de acceso de Xray y las muestra en forma de tabla: **Date, From, To, Inbound, Outbound, Email**.

A partir de la versión 3.4.1, esta ventana y el botón para abrirla en la tarjeta de estado de Xray se llaman **«Registros de acceso»** (`Access Logs`) — antes simplemente se llamaban «Registros». El cambio de nombre se realizó para no confundir el visor del registro de acceso de Xray con el visor de registros del propio panel, que antes tenía el mismo nombre.

| Elemento | Valor predeterminado | Descripción |
|----------|---------------------|-------------|
| Número de líneas | `20` | 20 / 50 / 100 / 500 / 1000 |
| **Filtro** | vacío | Búsqueda de texto por subcadena (se aplica al pulsar Enter) |
| **Actualización automática** (casilla) | desactivado | Volver a leer el registro cada 5 segundos (véase más abajo) |
| **Direct** (casilla) | activado | Mostrar conexiones directas (tráfico a través de freedom-outbound) |
| **Blocked** (casilla) | activado | Mostrar conexiones bloqueadas (tráfico al blackhole-outbound) |
| **Proxy** (casilla) | activado | Mostrar tráfico proxificado |

El tipo de evento se determina automáticamente por la etiqueta de la conexión saliente en la línea del registro: correspondencia con etiquetas freedom → «DIRECT» (verde), blackhole → «BLOCKED» (rojo), todo lo demás → «PROXY» (azul). Las líneas `api -> api` y las líneas vacías se omiten.

**Actualización automática.** En ambas ventanas de registros («Registros» y «Registros de acceso») hay una casilla **«Actualización automática»** (`Auto Update`). Si se activa, el contenido del registro se vuelve a leer automáticamente cada 5 segundos conservando toda la configuración actual de la ventana: número de líneas seleccionado, nivel/filtro y casillas Direct / Blocked / Proxy. El sondeo se detiene en cuanto se cierra la ventana o se desmarca la casilla.

> Para que esta ventana muestre entradas, Xray debe tener habilitado el **diario de acceso** con una ruta de archivo (no `none`) — véase más abajo. Si el registro de acceso está desactivado o el archivo no está disponible, la ventana estará vacía («No Record...»).

### 16.3. Nivel y configuración del registro de Xray

Los parámetros de registro del propio Xray se configuran en la página **«Configuraciones Xray»** en el bloque **«Registro»** (`Log`) con la advertencia:
> «Los registros pueden ralentizar el servidor. ¡Habilite solo los tipos de registro que necesita cuando sea necesario!»

| Campo | Nombre | Valor predeterminado | Descripción |
|-------|--------|---------------------|-------------|
| **Nivel de registros** (`logLevel`) | Log Level | `warning` | Nivel de detalle del registro de errores de Xray. Valores permitidos: `debug`, `info`, `notice`, `warning`, `error`. Ayuda: «Nivel de registro para los registros de errores, que indica la información que debe registrarse.» |
| **Registros de acceso** (`accessLog`) | Access Log | `none` | Ruta al archivo del diario de acceso. El valor especial `none` desactiva los registros de acceso. Ayuda: «Ruta al archivo del diario de acceso. El valor especial «none» desactiva los registros de acceso.» |
| **Registros de errores** (`errorLog`) | Error Log | vacío (ruta predeterminada) | Ruta al archivo de registros de errores; `none` los desactiva. Ayuda: «Ruta al archivo de registros de errores. El valor especial «none» desactiva los registros de errores.» |
| **Registros DNS** (`dnsLog`) | DNS Log | `false` (desact.) | Habilitar el registro de solicitudes DNS. Ayuda: «Habilitar registros de solicitudes DNS». |
| **Enmascaramiento de dirección** (`maskAddress`) | Mask Address | vacío (desact.) | Cuando está activo, la dirección IP real se reemplaza automáticamente por una de enmascaramiento en los registros. Ayuda: «Cuando está activo, la dirección IP real se reemplaza por una de enmascaramiento en los registros.» |

> Dado que de forma predeterminada **«Registros de acceso» = `none`**, la ventana «Registros de Xray» (sección 16.2) está inicialmente vacía. Para que funcione, establezca aquí la ruta al registro de acceso y reinicie Xray.

> Tenga en cuenta: el registro de acceso vacío solo afecta a esta ventana. La lista de clientes en línea en el «Dashboard» y el límite de cantidad de IP en el formulario del cliente **no dependen** del registro de acceso — el panel determina los clientes en línea y cuenta sus direcciones IP a través de la API de estadísticas en línea del núcleo Xray (estadísticas de conexiones). En versiones antiguas del núcleo donde esta API no existe, el panel vuelve automáticamente al método anterior (lectura del registro de acceso), y en ese caso la ruta al registro de acceso aquí sigue siendo necesaria para el límite de IP.

> **Límite de cantidad de IP y fail2ban.** La restricción por número de IP de un cliente (campo «IP Limit» en el formulario del cliente y al añadir en masa) se aplica en el servidor solo si **fail2ban** está instalado — es él quien bloquea las direcciones que superan el límite. El panel verifica la presencia de fail2ban (`GET /panel/api/server/fail2banStatus`); si no está instalado, el campo «IP Limit» queda deshabilitado con un mensaje explicativo (en Windows, con un mensaje separado), y los límites previamente establecidos en dichos servidores se anulan automáticamente, ya que de todos modos no estaban activos. El bloqueo de fail2ban se aplica tanto a TCP como a UDP. En los servidores habituales, fail2ban ahora se instala automáticamente durante la instalación y actualización del panel (véase la sección 16.5).

**Ejemplo: bloque `log` con el que la ventana «Registros de Xray» comenzará a mostrar entradas.** En la configuración JSON de Xray tiene este aspecto:

```json
{
  "log": {
    "loglevel": "warning",
    "access": "./access.log",
    "error": "",
    "dnsLog": false,
    "maskAddress": ""
  }
}
```

Lo principal es reemplazar `"access": "none"` por una ruta a un archivo (por ejemplo, `"./access.log"`). Después de guardar, reinicie Xray y la tabla en la ventana «Registros de Xray» comenzará a llenarse de líneas.

### 16.4. Gestión de Xray: detención y reinicio

El estado de Xray se gestiona desde la tarjeta de Xray en el «Dashboard». El estado actual se muestra con uno de estos valores: **Ejecutándose** (`Running`), **Detenido** (`Stopped`), **Desconocido** (`Unknown`), **Error** (`Error`). En caso de error, está disponible el aviso emergente «Error al iniciar Xray».

| Botón | Traducción | Endpoint | Acción |
|-------|------------|----------|--------|
| **Detener** | `Stop` | `POST /panel/api/server/stopXrayService` | Detiene el proceso de Xray. Si tiene éxito — notificación de aviso «Xray service has been stopped». |
| **Reiniciar** | `Restart` | `POST /panel/api/server/restartXrayService` | Reinicia (o inicia) Xray aplicando la configuración actual. Si tiene éxito — notificación «Xray service has been restarted successfully». |

Tras cualquiera de las operaciones, el panel difunde el nuevo estado por WebSocket, por lo que el estado en el «Dashboard» se actualiza sin recargar la página. Si la operación finaliza con error, el estado de Xray pasa a «Error» y el texto del error aparece en la notificación.

> Además del reinicio manual, el panel verifica automáticamente si es necesario reiniciar Xray (tarea en segundo plano cada 30 s) y si el proceso ha fallado (comprobación cada segundo) — véase la sección 16.6.

#### Monitor de salud del túnel (reinicio automático de Xray)

En la versión 3.4.1 se introdujo un **monitor de salud del túnel** opcional. Si está habilitado, el panel verifica periódicamente la disponibilidad de una URL determinada y, tras varios intentos fallidos consecutivos, reinicia automáticamente el núcleo Xray — esto ayuda a recuperar un túnel que ha dejado de transmitir tráfico. De forma predeterminada, el monitor está **desactivado** y se configura **únicamente mediante variables de entorno del servicio** (no tiene configuración en la interfaz web — así lo diseñaron los autores).

El monitor se activa con la variable `XUI_TUNNEL_HEALTH_MONITOR=true`. La variable `XUI_TUNNEL_HEALTH_PROXY` debe apuntar a un inbound local de xray (por ejemplo `socks5://127.0.0.1:1080`) — de este modo, la sonda pasa a través del propio Xray y verifica específicamente el túnel; sin ella solo se comprueba la conectividad del host, y el reinicio no resolverá un problema de conectividad de red del servidor. El resto de variables definen los parámetros de la comprobación:

| Variable | Propósito | Predeterminado |
|----------|-----------|---------------|
| `XUI_TUNNEL_HEALTH_MONITOR` | Habilitar el monitor (act./desact.) | `false` |
| `XUI_TUNNEL_HEALTH_PROXY` | Proxy a través del cual se realiza la sonda (indique un inbound local de xray) | vacío |
| `XUI_TUNNEL_HEALTH_URL` | URL que se comprueba | `https://www.cloudflare.com/cdn-cgi/trace` |
| `XUI_TUNNEL_HEALTH_INTERVAL` | Intervalo entre comprobaciones | `30s` |
| `XUI_TUNNEL_HEALTH_TIMEOUT` | Tiempo de espera de una comprobación | `10s` |
| `XUI_TUNNEL_HEALTH_FAILURES` | Número de fallos consecutivos antes del reinicio | `3` |
| `XUI_TUNNEL_HEALTH_COOLDOWN` | Pausa mínima entre reinicios | `5m` |

> El reinicio de Xray interrumpe las conexiones de todos los clientes conectados, por lo que conviene mantener el intervalo y el umbral de fallos suficientemente altos para que un fallo accidental de una sonda no provoque reinicios innecesarios.

### 16.5. Reinicio y actualización del panel

#### Reinicio del panel

En la página **«Configuración del panel»** hay una acción **«Reiniciar panel»** (`Restart Panel`, `POST /panel/api/setting/restartPanel`). Al confirmarlo, el panel se reinicia **en 3 segundos**.

Mensajes:
- Confirmación: «¿Está seguro de que desea reiniciar el panel? Confirme y el reinicio se producirá en 3 segundos. Si el panel no está disponible, compruebe el registro del servidor.»
- Éxito: «El panel se ha reiniciado correctamente».

Técnicamente, en Linux el reinicio se realiza enviando la señal `SIGHUP` al proceso del panel (o a través del gancho registrado). En Windows el envío de `SIGHUP` no está soportado.

#### Actualización automática del panel (Update Panel)

En el «Dashboard» está disponible la función **«Actualizar panel»** (`Update Panel`) — actualización de 3X-UI a la última versión directamente desde la interfaz web.

Antes de actualizar, el panel compara las versiones (`GET /panel/api/server/getPanelUpdateInfo`), consultando la última versión de 3x-ui en GitHub:

| Campo | Traducción |
|-------|------------|
| **Versión actual del panel** | Current panel version |
| **Última versión del panel** | Latest panel version |
| **Panel actualizado** / «Actualizado» | Panel is up to date / Up to date — se muestra si no hay nueva versión |

Inicio de la actualización — `POST /panel/api/server/updatePanel`. Diálogo de confirmación:
- «¿Realmente desea actualizar el panel?»
- «Esto actualizará 3X-UI a la versión #version# y reiniciará el servicio del panel.»

Tras el inicio — mensaje emergente «Actualización del panel iniciada» (`Panel update started`); si la comprobación de versiones falla — «La comprobación de actualización del panel falló» (`Panel update check failed`).

**Qué ocurre en el servidor:** la actualización automática solo está soportada **en Linux** (en otros sistemas operativos se devolverá el error «panel web update is supported only on Linux installations»). El panel descarga el script oficial `update.sh` de GitHub (`raw.githubusercontent.com/MHSanaei/3x-ui/main/update.sh`) y lo ejecuta en un proceso separado: preferiblemente a través de `systemd-run` en una unidad separada (`x-ui-web-update-<timestamp>`), y si systemd no está disponible, como un proceso separado desconectado. Al finalizar, el script actualiza los componentes y reinicia el servicio del panel. Para ejecutarse requiere `bash`.

Si durante la actualización el script generó una nueva ruta base aleatoria de la interfaz web (Web Base Path), el servicio `x-ui` se reinicia automáticamente para que la nueva ruta funcione de inmediato. (Sin el reinicio, el servidor seguiría sirviendo la ruta antigua mientras la interfaz mostraba la nueva, y la nueva dirección no estaría disponible hasta un reinicio manual.)

#### Canal de actualización Dev (compilaciones rolling por commit)

Además de la actualización habitual a la versión estable, existe un **«Canal de desarrollo»** opcional (`Dev`). El interruptor aparece en la ventana de actualización del panel **solo en compilaciones dev** (compilaciones de CI generadas por un commit específico); en las versiones estables no es visible. Cuando está activado, el panel se actualizará a la compilación rolling `dev-latest`, que rastrea cada commit de la rama `main` y no es una versión estable — se muestra una advertencia de que las compilaciones dev son inestables y no hay reversión automática. En modo dev, la ventana muestra «Commit actual» / «Último commit» en lugar de números de versión. La función solo está disponible en Linux con systemd.

En las compilaciones dev, el panel muestra su versión como `dev+<commit-corto>` en lugar del número de versión estable que podría confundir — en el distintivo de la barra lateral, en la tarjeta del «Dashboard», en la ventana de actualización, en el informe de estado del bot de Telegram y en la salida del comando `x-ui -v`. En las versiones estables, el formato de versión no cambia.

> En los nodos, el panel del mismo 3x-ui se actualiza de forma centralizada a través de `POST /panel/api/nodes/updatePanel` — véase la sección sobre nodos.

#### Instalación automática de fail2ban

Para que el límite de cantidad de IP de los clientes (sección 16.3) funcione desde el primer momento, al instalar y actualizar el panel en un servidor convencional `fail2ban` ahora se instala y configura automáticamente (antes esto solo ocurría en la imagen Docker). El comportamiento lo controla la variable de entorno `XUI_ENABLE_FAIL2BAN`: la configuración se realiza si la variable no está definida o es igual a `true`. La ejecución manual está disponible con el comando `x-ui setup-fail2ban`. Un fallo en la configuración de fail2ban no interrumpe la instalación o actualización del panel.

#### Instalación y actualización en hosts con solo IPv6

Los scripts `install.sh` y `update.sh` ahora funcionan correctamente en servidores solo con IPv6: la descarga de la versión, el script `x-ui.sh` y los archivos de servicio ya no usa IPv4 forzado (`curl -4`), sino el protocolo disponible. Por lo tanto, el panel puede instalarse y actualizarse también en un host sin dirección IPv4.

#### Sobreescritura del puerto del panel con la variable `XUI_PORT`

El puerto de escucha de la interfaz web puede sobreescribirse con la variable de entorno `XUI_PORT` — esta actúa solo durante la ejecución del proceso actual y **no modifica** el valor guardado `webPort` en la base de datos. Se aceptan valores de `1` a `65535`; un valor vacío, incorrecto o fuera del rango se ignora (se usa `webPort`) con una advertencia en el registro. Esto es conveniente durante el despliegue, principalmente en Docker: al usar una red bridge, el puerto publicado del contenedor debe coincidir con `XUI_PORT` — por ejemplo, `XUI_PORT=8080` y `ports: "8080:8080"`.

#### Actualización y cambio de versión de Xray-core

También desde el «Dashboard» se puede gestionar la versión de Xray-core de forma independiente al panel.

- **Actualizaciones de Xray** (`Xray Updates`) / **Seleccionar versión** (`Version`) — lista desplegable de versiones disponibles. Mensajes de ayuda: «Seleccione la versión deseada» y la advertencia «Importante: las versiones antiguas pueden no ser compatibles con la configuración actual».
- Instalación/cambio de versión — `POST /panel/api/server/installXray/{version}`. Diálogo: «Cambiar versión de Xray» / «¿Está seguro de que desea cambiar la versión de Xray?». Si tiene éxito — «Xray actualizado correctamente».

**Ejemplo: cambio de versión de Xray-core mediante solicitud a la API.** La versión se especifica con la etiqueta de versión de XTLS/Xray-core (con el prefijo `v`). Por ejemplo, cambiar a `v1.8.24`:

```bash
curl -s -b cookies.txt -X POST \
     https://panel.example.com:2053/panel/api/server/installXray/v1.8.24
```

(`cookies.txt` — archivo con la cookie del ejemplo en la sección 16.1.) Tras la instalación, Xray se reiniciará automáticamente con la versión seleccionada.

En el servidor, al cambiar la versión, Xray primero se detiene, se descarga el archivo de la versión necesaria desde GitHub (XTLS/Xray-core), el binario se descomprime y se reemplaza, y luego Xray se reinicia verificando los tamaños de control del archivo/binario.

### 16.6. Tareas periódicas (cron)

El panel registra una serie de tareas en segundo plano al iniciarse. Sus programaciones están fijas (no configurables en la interfaz, excepto la programación del informe de Telegram y la sincronización LDAP). A continuación se muestran las tareas relacionadas con la operación.

| Tarea | Programación | Propósito |
|-------|-------------|-----------|
| Comprobación del funcionamiento de Xray | cada 1 s | Control de que el proceso Xray está en ejecución |
| Comprobación de necesidad de reinicio de Xray | cada 30 s | Reinicio si la configuración está marcada como modificada |
| Recopilación de tráfico de Xray | cada 5 s (inicio 5 s después del arranque) | Contabilización del tráfico de inbound/clientes |
| Comprobación de IP de clientes | cada 10 s | Control del límite de IP por registro |
| Heartbeat y sincronización de tráfico de nodos | cada 5 s | Intercambio con nodos |
| **Limpieza de registros** | **diariamente** (`@daily`) | Limpia los registros de límite de IP y el registro de acceso persistente, rotando el registro actual a `*.prev.log` |
| **Restablecimiento de tráfico por período** | `@hourly`, `@daily`, `@weekly`, `@monthly` | Restablece los contadores de tráfico de los inbound (y sus clientes) que tienen configurado el período de restablecimiento automático correspondiente |
| Informe de Telegram | se establece en la configuración del bot (predeterminado `@daily`) | Envío del informe a los administradores; si la opción está habilitada — con la copia de seguridad de la BD adjunta (sección 16.1) |
| Restablecimiento del almacenamiento hash de Telegram | cada 2 m | Solo con el bot habilitado |
| Control de carga de CPU para Telegram | cada 10 s | Solo si el umbral de CPU > 0 está establecido |

Adicionalmente:

- **El restablecimiento periódico de tráfico** solo se activa para los inbound que tienen seleccionado el modo de restablecimiento automático correspondiente (cada hora/día/semana/mes). La tarea restablece el tráfico del propio inbound y de todos sus clientes.
- **Comprobación de vencimiento y agotamiento.** La desactivación de clientes por vencimiento del plazo y agotamiento del límite de tráfico se realiza durante la contabilización del tráfico: los clientes con `expiry_time` vencido o volumen agotado se marcan y desactivan; si es necesario, se calcula el siguiente plazo (para límites cíclicos y el modo «cuenta desde el primer uso»). En el «Dashboard» y las listas esto se refleja con los estados «Vencido»/«Agotado»/«Próximo a vencer».
- **La copia de seguridad automática en Telegram** es un efecto secundario de la tarea del informe; no hay una programación cron separada solo para la copia de seguridad. Por tanto, la frecuencia de la copia de seguridad automática es igual a la frecuencia del informe del bot.

### 16.7. Menú de consola y CLI (`x-ui`)

En el servidor, el panel se gestiona con el comando `x-ui`. Sin argumentos, se abre el menú interactivo «3X-UI Panel Management Script»; con un argumento, se ejecuta un subcomando específico. Los elementos del menú relacionados con la operación:

| N.º en el menú | Elemento | Acción |
|----------------|---------|--------|
| 1 | Install | Instalación del panel (descarga y ejecuta `install.sh`) |
| 2 | Update | Actualización de todos los componentes de x-ui a la última versión sin pérdida de datos; después — reinicio automático |
| 3 | Update to Dev Channel (latest commit) | Actualización a la compilación rolling `dev-latest` (último commit de la rama `main`) con confirmación (véase 16.5) |
| 4 | Update Menu | Actualización solo del propio script de menú `x-ui` |
| 5 | Legacy Version | Instalación de la versión indicada (antigua) del panel por el número introducido (por ejemplo, `2.4.0`) |
| 6 | Uninstall | Eliminación completa del panel y Xray (véase 16.8) |
| 7 | Reset Username & Password | Restablecimiento del nombre de usuario/contraseña del administrador |
| 8 | Reset Web Base Path | Restablecimiento de la ruta base de la interfaz web |
| 9 | Reset Settings | Restablecimiento de la configuración a los valores predeterminados |
| 10 | Change Port | Cambio del puerto del panel |
| 11 | View Current Settings | Visualización de la configuración actual |
| 12–14 | Start / Stop / Restart | Inicio, detención, reinicio del servicio del panel |
| 15 | Restart Xray | Reinicio solo de Xray |
| 16 | Check Status | Estado actual del servicio |
| 17 | Logs Management | Visualización y limpieza de registros (véase más abajo) |
| 18–19 | Enable / Disable Autostart | Habilitar/deshabilitar el inicio automático del servicio al arrancar el SO |
| 27 | Update Geo Files | Actualización de archivos geográficos (GeoIP/GeoSite) |
| 25 | PostgreSQL Management | Gestión de PostgreSQL |

> La numeración de los elementos del menú cambió en la versión 3.4.1: con la adición del elemento 3 «Update to Dev Channel», todos los elementos posteriores se desplazaron en uno. El total de elementos es ahora 28, y la selección se introduce en el rango `[0-28]`.

#### Gestión de registros en la CLI (elemento 16)

El submenú «Logs Management» ahora se abre con el elemento **17** (antes — 16):
- **Debug Log** — visualización en tiempo real del diario del servicio: `journalctl -u x-ui -e --no-pager -f -p debug` (en Alpine — `grep` sobre `/var/log/messages`).
- **Clear All logs** — limpieza del diario del sistema: `journalctl --rotate` + `journalctl --vacuum-time=1s`, tras lo cual el servicio se reinicia. (No disponible en Alpine.)

#### Subcomandos directos de `x-ui`

Todos los subcomandos disponibles:

| Comando | Descripción |
|---------|-------------|
| `x-ui` | Abrir el menú de administración |
| `x-ui start` | Iniciar el panel |
| `x-ui stop` | Detener el panel |
| `x-ui restart` | Reiniciar el panel |
| `x-ui restart-xray` | Reiniciar Xray |
| `x-ui status` | Estado actual |
| `x-ui settings` | Mostrar la configuración actual |
| `x-ui enable` | Habilitar el inicio automático al arrancar el SO |
| `x-ui disable` | Deshabilitar el inicio automático |
| `x-ui log` | Visualizar registros |
| `x-ui banlog` | Visualizar registros de bloqueos de Fail2ban |
| `x-ui setup-fail2ban` | Instalar y configurar fail2ban para el límite de IP (véase 16.5) |
| `x-ui update` | Actualizar el panel |

| `x-ui update-dev` | Actualizar el panel al canal de desarrollo (compilación rolling `dev-latest`) |
| `x-ui update-all-geofiles` | Actualizar todos los archivos geográficos (con reinicio posterior) |
| `x-ui migrateDB [file]` | Conversión de base de datos `.db ⇄ .dump` (SQLite) |
| `x-ui legacy` | Instalar una versión obsoleta |
| `x-ui install` | Instalar el panel |
| `x-ui uninstall` | Eliminar el panel |

> El comando `x-ui update` descarga y ejecuta el `update.sh` oficial (el mismo que la actualización web de la sección 16.5), solicitando confirmación: «This function will update all x-ui components to the latest version, and the data will not be lost.» Al finalizar, el panel se reinicia automáticamente.

> **Indicadores `-webCert` / `-webCertKey` en el subcomando `setting`.** Las rutas al certificado y la clave privada de la interfaz web pueden establecerse directamente en el subcomando `x-ui setting -webCert <ruta> -webCertKey <ruta>` — especificar cualquiera de estos indicadores guarda la ruta correspondiente (igual que el subcomando separado `cert`), y el panel pasa inmediatamente a HTTPS.

#### Obtención del token de API mediante la CLI

El comando de obtención del token de API mediante la CLI (elemento del menú/comando `x-ui`) no muestra el token emitido anteriormente. Los tokens de API se almacenan solo en forma de hashes, por lo que un token existente no puede obtenerse en texto claro. Si ya hay tokens configurados, el comando informa de su cantidad, recomienda gestionarlos en el panel (**Settings → API Tokens**, véase la sección sobre tokens de API) y genera inmediatamente un **nuevo token de reserva** con el nombre del tipo `cli-fallback-<timestamp>` y lo muestra, para que la CLI siga siendo útil sin acceder a la interfaz.

### 16.8. Eliminación del panel

La eliminación se realiza desde la CLI — elemento de menú **5 (Uninstall)** o comando `x-ui uninstall`. Antes de eliminar se solicita confirmación (predeterminado «no»): «Are you sure you want to uninstall the panel? xray will also uninstalled!».

Al confirmar, el script:
1. Detiene el servicio y deshabilita su inicio automático (`systemctl stop/disable x-ui`, o en Alpine — `rc-service`/`rc-update`), elimina el archivo de unidad del servicio y recarga la configuración de systemd.
2. Elimina los directorios de datos y de la aplicación (`/etc/x-ui/`, directorio de instalación) y el archivo de entorno del servicio (`/etc/default/x-ui`, `/etc/conf.d/x-ui` o `/etc/sysconfig/x-ui` — según la distribución).
3. Elimina el propio script `x-ui` y muestra el mensaje «Uninstalled Successfully.», así como el comando para reinstalarlo.

Si el panel usaba PostgreSQL (en el archivo de entorno `XUI_DB_TYPE=postgres`), después de eliminar los archivos del panel, el script pregunta adicionalmente si es necesario eliminar también el propio servidor PostgreSQL junto con todas sus bases de datos: «Also purge PostgreSQL and delete all of its data?». La solicitud requiere confirmación explícita (predeterminado — negativa) y va acompañada de una advertencia: la eliminación afectará a **TODAS** las bases de datos de PostgreSQL en la máquina, incluidas las que pertenecen a otras aplicaciones, y es irreversible. Si se rechaza, PostgreSQL y sus datos no se modifican.

> La eliminación es irreversible: junto con el panel se elimina Xray y todos los datos (incluida la base de datos). Si puede necesitar los datos, exporte la base de datos previamente (sección 16.1).

### 16.9. Comando `x-ui migrateDB`

A partir de la versión 3.3.0, el script de gestión `x-ui.sh` recibió el subcomando `migrateDB` — un envoltorio alrededor del binario integrado `x-ui` (`x-ui migrate-db`) para convertir la base de datos del panel SQLite entre dos formatos: el binario `.db` y el volcado de texto portable `.dump` (texto SQL ordinario).

#### Qué hace el comando

El comando funciona en dos direcciones, y la dirección se determina **automáticamente** según el archivo de entrada:

| Dirección | Cómo se llama | Qué ocurre |
|---|---|---|
| `.db → .dump` | dump (volcado) | la base SQLite binaria se vuelca en un archivo SQL de texto |
| `.dump → .db` | restore (restauración) | a partir del archivo SQL de texto se reconstruye la base SQLite binaria |

Internamente, el script llama al binario del panel:
- volcado: `x-ui migrate-db --src <entrada> --dump <salida>`
- restauración: `x-ui migrate-db --restore <entrada> --out <salida>`

#### Sintaxis de uso

```
x-ui migrateDB [file.db|file.dump] [output]
```

- **`[file.db|file.dump]`** — archivo de entrada (primer argumento). Si no se especifica, se usa la base de datos instalada del panel por defecto: `/etc/x-ui/x-ui.db`.
- **`[output]`** — ruta al archivo de salida (segundo argumento). Opcional: si no se indica, el nombre se elige automáticamente junto al archivo de entrada (véase más abajo).

Ejemplos:

```
x-ui migrateDB                              # volcar /etc/x-ui/x-ui.db -> /etc/x-ui/x-ui.dump
x-ui migrateDB /etc/x-ui/x-ui.db backup.dump
x-ui migrateDB backup.dump restored.db      # reconstruir .db desde el volcado
```

#### Cómo se determina la dirección

El script analiza la extensión del archivo de entrada:
- `*.db`, `*.sqlite`, `*.sqlite3` → modo **dump** (volcado a texto);
- `*.dump`, `*.sql` → modo **restore** (construcción de la base).

Si la extensión no se reconoce, el script lee los primeros 16 bytes del archivo: la firma `SQLite format 3` indica una base binaria (modo dump), de lo contrario el archivo se trata como un volcado (modo restore).

El nombre del archivo de salida, si no se especifica el segundo argumento:
- en el volcado — el mismo nombre que la entrada, con la extensión `.dump`;
- en la restauración — el mismo nombre con la extensión `.db`.

#### Verificaciones de protección y comportamiento

- **Presencia del binario.** Si el binario `x-ui` no se encuentra o no es ejecutable — se muestra el error «x-ui binary not found … Is the panel installed?».
- **Compatibilidad de la función en la compilación.** El script verifica que el binario admite `migrate-db --dump/--restore` (mediante `x-ui migrate-db -h`). Si no — se sugiere primero actualizar el panel con `x-ui update`.
- **Existencia del archivo de entrada.** Si el archivo de entrada no existe, se imprime un error y la línea con la sintaxis de uso.
- **Sobreescritura de la salida.** Si el archivo de salida ya existe, se solicita confirmación (predeterminado «no»); sin confirmación, la operación se cancela. En la restauración, el archivo de salida anterior se elimina previamente.
- **Protección de la base «en vivo».** Al restaurar en la base de datos predeterminada `/etc/x-ui/x-ui.db` cuando el panel está en ejecución, la operación se rechaza con la petición de detener primero el panel (`x-ui stop`) o elegir otra ruta de salida. Esto evita sobrescribir la base de datos activa del servicio en funcionamiento.
- Si la construcción de la base falla, el archivo de salida incompleto se elimina.

#### Para qué sirve

- **Copia de seguridad.** El `.dump` de texto es legible por humanos, conveniente para almacenar en sistemas de control de versiones y para comparar el contenido de la base de datos.
- **Migración.** El volcado es portable entre máquinas y resistente a diferencias en las versiones del formato del archivo SQLite — en el nuevo servidor se construye una `.db` funcional a partir de él.
- **Diagnóstico.** Desde el `.dump` se puede examinar visualmente la estructura y los datos del panel sin tener a mano herramientas SQLite.

#### Modo interactivo

Además de la invocación directa, la conversión está disponible desde el menú interactivo. En el submenú PostgreSQL (`x-ui` → sección de gestión de PostgreSQL) hay el elemento **9. Convert SQLite `.db <-> .dump`**: solicita la ruta al archivo de entrada (predeterminado `/etc/x-ui/x-ui.db`) y al archivo de salida (puede dejarse vacío para el nombramiento automático), y la dirección, al igual que en el modo CLI, se determina automáticamente.

---

*Documento preparado a partir del código fuente de 3X-UI. Si algún elemento de la interfaz en su versión es diferente — prevalece el comportamiento del panel y los mensajes de ayuda en el propio UI.*
