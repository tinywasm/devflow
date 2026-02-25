# **Análisis integral de los sistemas de webhooks en el ecosistema de GitHub: Arquitectura, economía, implementación y ciclo de vida operativo**

La integración de sistemas en el desarrollo de software moderno ha transitado desde modelos de consulta pasiva hacia arquitecturas reactivas impulsadas por eventos. En este paradigma, los webhooks de GitHub se consolidan como el pilar fundamental para la automatización de flujos de trabajo, permitiendo que aplicaciones externas reciban notificaciones en tiempo real cuando ocurren eventos específicos dentro de la plataforma.1 Este mecanismo elimina la ineficiencia inherente al *polling* de APIs, donde un cliente debe solicitar información de manera intermitente para detectar cambios, optimizando así el uso de recursos computacionales y mejorando la escalabilidad de las integraciones.1

## **Fundamentos operativos y mecánica de los webhooks**

El funcionamiento de un webhook de GitHub se basa en un modelo de suscripción donde el usuario define un punto de enlace (endpoint) HTTP y selecciona una serie de eventos de interés.1 Cuando se produce uno de estos eventos —como la apertura de un *pull request*, un *push* de código o la creación de un problema—, GitHub genera una solicitud HTTP POST dirigida a la URL especificada.1 Esta solicitud transporta una carga útil (payload) en formato JSON o codificado como formulario, que contiene información detallada y estructurada sobre el evento ocurrido.2

### **Protocolos de comunicación y cabeceras técnicas**

La comunicación entre GitHub y el servidor receptor se gestiona a través del protocolo HTTP/S. Para facilitar la identificación y el procesamiento de las entregas sin necesidad de analizar inmediatamente el cuerpo del mensaje, GitHub incluye una serie de cabeceras personalizadas en cada solicitud.2

| Cabecera HTTP | Función y Relevancia Técnica |
| :---- | :---- |
| X-GitHub-Event | Indica el tipo de evento que disparó el webhook (ej. push, issues, pull\_request).2 |
| X-GitHub-Delivery | Proporciona un identificador único global (GUID) para la entrega, permitiendo el rastreo y la deduplicación.2 |
| X-Hub-Signature-256 | Firma criptográfica HMAC-SHA256 del payload, esencial para verificar la autenticidad de la fuente.7 |
| User-Agent | Identifica el cliente remitente, que en este caso siempre comienza con la cadena GitHub-Hookshot/.2 |
| X-GitHub-Hook-ID | El identificador único del webhook configurado en GitHub.10 |

El uso de la cabecera X-GitHub-Delivery resulta crítico en entornos de alta disponibilidad. Dado que los sistemas distribuidos pueden experimentar reintentos de red o latencias, es posible que un servidor reciba la misma entrega más de una vez. La implementación de una lógica de idempotencia, que verifique si el GUID ya ha sido procesado antes de ejecutar acciones costosas, es una práctica recomendada para mantener la integridad del sistema.2

### **Tipología de webhooks y alcances de suscripción**

GitHub permite la creación de webhooks en diferentes niveles de su jerarquía organizativa, lo que determina el alcance de los eventos que el sistema puede capturar y los permisos necesarios para su gestión.3

1. **Webhooks de Repositorio:** Son los más comunes y se limitan a eventos que ocurren dentro de un proyecto específico. Requieren permisos de administrador sobre el repositorio.3  
2. **Webhooks de Organización:** Capturan eventos en todos los repositorios pertenecientes a una organización, así como eventos a nivel de organización (ej. adición de miembros o creación de equipos). Solo pueden ser gestionados por los propietarios de la organización.3  
3. **Webhooks de GitHub Apps:** Permiten a los desarrolladores de aplicaciones recibir eventos de todas las instalaciones de su aplicación. Son fundamentales para construir integraciones que operan a gran escala en múltiples cuentas.3  
4. **Webhooks de Marketplace y Sponsors:** Específicos para eventos relacionados con transacciones comerciales o patrocinios, permitiendo a los creadores reaccionar ante cambios en sus planes de suscripción o apoyo financiero.3

## **Análisis económico y modelos de costos asociados**

Desde una perspectiva estrictamente transaccional, GitHub no cobra una tarifa por la configuración o el uso de webhooks.13 Esta funcionalidad es accesible para todos los usuarios, incluyendo aquellos en el plan gratuito.15 Sin embargo, el despliegue de una arquitectura basada en webhooks conlleva costos indirectos significativos que deben ser evaluados por los arquitectos de sistemas.13

### **Infraestructura de recepción y procesamiento**

El costo principal reside en el servidor o servicio encargado de recibir y procesar las solicitudes POST de GitHub. Dependiendo de la carga de trabajo y los requisitos de disponibilidad, las organizaciones pueden optar por diversas soluciones que varían en costo operativo.13

| Componente de Infraestructura | Modelo de Costo Estimado | Consideraciones Técnicas |
| :---- | :---- | :---- |
| Instancias de Nube (EC2/GCE) | Pago por hora (desde $0.18/hr para 2 núcleos).13 | Requiere gestión de parches y escalado manual.19 |
| Contenedores (Fargate/Cloud Run) | Pago por recursos consumidos y solicitudes. | Ideal para cargas variables y microservicios.19 |
| Servicios Especializados (Hookdeck) | Planes mensuales basados en volumen de eventos. | Proporciona colas de mensajes y gestión de errores nativa.5 |
| Ancho de Banda (Egress) | Aproximadamente $0.0875 \- $0.50 por GB.13 | Crítico si los webhooks transfieren grandes payloads.8 |

### **Relación con los límites de GitHub Actions**

Los webhooks a menudo actúan como el disparador (trigger) para flujos de trabajo en GitHub Actions. Aunque el webhook es gratuito, la ejecución del flujo de trabajo consume minutos de computación que están sujetos a los límites del plan del usuario.15

| Plan de GitHub | Minutos de Actions Incluidos | Almacenamiento Incluido | Concurrencia Máxima |
| :---- | :---- | :---- | :---- |
| **GitHub Free** | 2,000 minutos/mes.15 | 500 MB.15 | 20 trabajos.22 |
| **GitHub Pro** | 3,000 minutos/mes.16 | 1 GB.22 | 40 trabajos.22 |
| **GitHub Team** | 3,000 minutos/mes.16 | 2 GB.22 | 60 trabajos.22 |
| **GitHub Enterprise** | 50,000 minutos/mes.15 | 50 GB.15 | 500 trabajos.22 |

Cualquier uso que exceda estos límites se factura según la tarifa correspondiente al tipo de ejecutor (runner). Por ejemplo, un ejecutor estándar de Linux cuesta aproximadamente $0.008 por minuto, mientras que opciones más potentes pueden elevar este costo hasta $0.162 por minuto para configuraciones de 64 núcleos.13

## **Implementación técnica y protocolos de configuración**

La implementación exitosa de un webhook requiere una coordinación precisa entre la interfaz de GitHub y el entorno de servidor del desarrollador. Este proceso puede automatizarse mediante la API REST de GitHub o realizarse manualmente a través de la consola de administración.3

### **Configuración en la plataforma GitHub**

Para establecer un nuevo webhook en un repositorio, el administrador debe navegar a la sección de configuración y definir los parámetros operativos fundamentales.2

1. **URL de Carga (Payload URL):** Es la dirección pública del servidor receptor. Se recomienda encarecidamente el uso de HTTPS con certificados válidos para proteger los datos en tránsito.3  
2. **Tipo de Contenido (Content Type):** GitHub ofrece dos opciones principales: application/json, que entrega el payload directamente en el cuerpo de la solicitud POST, y application/x-www-form-urlencoded, que lo envía como un parámetro de formulario llamado payload.3  
3. **Secreto (Secret):** Una cadena de caracteres de alta entropía que actúa como clave compartida. GitHub la utiliza para firmar el payload mediante HMAC, permitiendo al receptor validar que la solicitud proviene efectivamente de GitHub y no ha sido alterada.3  
4. **Selección de Eventos:** El sistema permite suscribirse a todos los eventos disponibles, solo al evento push o a una selección granular de eventos individuales.2 La mejor práctica dicta suscribirse únicamente a los eventos necesarios para reducir el ruido y la carga en el servidor.2

### **Desarrollo del servidor receptor**

El servidor encargado de escuchar las notificaciones debe cumplir con requisitos técnicos estrictos para garantizar una integración fluida. La arquitectura más robusta implica el uso de un servidor web (como Nginx o Apache) actuando como proxy inverso frente a una aplicación escrita en lenguajes como Go, Python, Node.js o Ruby.19

El flujo lógico del servidor receptor debe seguir estos pasos 2:

* **Escucha de Peticiones:** El servidor debe estar activo y escuchando en el puerto y la ruta configurados (ej. midominio.com/webhooks/github).  
* **Lectura de Cabeceras:** Extraer X-GitHub-Event y X-Hub-Signature-256 para identificar la acción y prepararse para la validación.  
* **Validación de Firma:** Calcular el hash HMAC-SHA256 del cuerpo bruto de la solicitud utilizando el secreto configurado y compararlo con la firma recibida.2  
* **Respuesta Inmediata:** Devolver un código de estado 2xx dentro de los primeros 10 segundos para confirmar la recepción.2  
* **Procesamiento Asíncrono:** Delegar la lógica de negocio (ej. disparar un despliegue) a una cola de tareas para no bloquear la respuesta al webhook.2

## **Ciclo de vida de una entrega y gestión de fiabilidad**

El ciclo de vida de un webhook comienza con una acción en GitHub y termina con la resolución exitosa o el registro de un fallo en el servidor del usuario. Comprender cada etapa es vital para el diagnóstico y la resolución de problemas.4

### **Flujo secuencial de la entrega**

Cuando ocurre un evento suscrito, GitHub encola la entrega para su envío inmediato. El proceso sigue una secuencia temporal definida por las políticas de la plataforma.2

| Etapa del Ciclo | Acción Realizada | Responsabilidad |
| :---- | :---- | :---- |
| **Disparo (Trigger)** | Se produce el evento en GitHub (ej. *commit*). | GitHub.1 |
| **Preparación** | Generación del JSON y firma HMAC del payload.7 | GitHub.7 |
| **Transmisión** | Envío de la solicitud POST al endpoint configurado.1 | GitHub.1 |
| **Recepción** | Validación de firma y almacenamiento inicial del payload.7 | Usuario.2 |
| **Confirmación** | Envío de respuesta HTTP 2xx (antes de 10 segundos).4 | Usuario.2 |
| **Ejecución** | Procesamiento de la lógica de negocio en segundo plano.2 | Usuario.5 |

### **Políticas de reintento y redelivación**

Un aspecto crítico que diferencia a GitHub de otros proveedores de webhooks es su política de reintentos. Oficialmente, GitHub no realiza reintentos automáticos para la mayoría de las entregas fallidas en repositorios y organizaciones.28 Si el servidor del usuario está caído o devuelve un error, la entrega se marca como fallida en el registro histórico y no se vuelve a intentar de forma proactiva por parte de la infraestructura de GitHub.27

Sin embargo, existen mecanismos para recuperar entregas perdidas 12:

* **Redelivación Manual:** Los usuarios pueden acceder a la pestaña "Recent Deliveries" en la configuración del webhook y solicitar el reenvío de cualquier entrega fallida de los últimos 3 días.12  
* **Automatización vía API:** Es posible desarrollar scripts que consulten periódicamente el endpoint de entregas de la API de GitHub, identifiquen aquellas con un estado que no sea "OK" y soliciten su redelivación automática enviando una solicitud POST al endpoint /attempts.28  
* **Deduplicación:** Al redelivarse, GitHub mantiene el mismo X-GitHub-Delivery GUID original. Esto permite al servidor receptor identificar que se trata de un reenvío y evitar el procesamiento duplicado de la misma acción.5

## **Infraestructura de envío y recepción: Linux y Contenedores**

La infraestructura que sustenta los webhooks es una combinación de sistemas propietarios de GitHub para la emisión y herramientas de código abierto para la recepción, predominantemente basadas en Linux y tecnologías de contenedores.19

### **El emisor: GitHub Hookshot**

El componente encargado de despachar las notificaciones desde la red interna de GitHub hacia el internet público se conoce como **Hookshot**.2 Aunque GitHub no publica detalles íntimos de su sistema operativo, la cabecera User-Agent revela la identidad de este agente.2

Para garantizar que los webhooks lleguen a su destino en redes empresariales restringidas, los administradores deben permitir el tráfico entrante desde los rangos de direcciones IP específicos que GitHub dedica a este servicio.5 Estos rangos se pueden obtener consultando el endpoint /meta de la API de GitHub, que devuelve un objeto JSON con las redes autorizadas en formato CIDR.30 Es imperativo monitorizar este endpoint regularmente, ya que GitHub actualiza sus direcciones IP periódicamente para escalar su capacidad.5

### **El receptor: Herramientas en Linux y Docker**

En el lado del receptor, la flexibilidad de Linux permite implementar soluciones ligeras y altamente configurables. Una de las herramientas más extendidas es webhook, un servidor escrito en Go que permite mapear endpoints HTTP directamente a la ejecución de scripts o binarios en el sistema local.19

#### **Implementación con webhook-go en Linux**

Esta utilidad destaca por su simplicidad y su capacidad para integrarse con flujos de trabajo de shell existentes.19

* **Instalación:** Disponible de forma nativa en distribuciones como Ubuntu y Debian (sudo apt-get install webhook).25  
* **Funcionamiento:** Escucha peticiones POST, valida reglas opcionales (como la presencia de un token secreto) y pasa los datos del payload como argumentos o variables de entorno a los comandos configurados.19  
* **Arquitectura de Producción:** Se suele desplegar detrás de un proxy inverso como Nginx para gestionar la seguridad TLS y el balanceo de carga.25

#### **Contenedores y Microservicios**

El uso de Docker se ha convertido en el estándar para desplegar receptores de webhooks debido a su capacidad de aislamiento y facilidad de orquestación.19

* **Imagen linuxserver/webhook:** Una de las implementaciones más populares en el ecosistema de contenedores, que soporta múltiples arquitecturas (x86-64, arm64) y permite una configuración sencilla a través de volúmenes para los scripts y archivos de configuración JSON/YAML.19  
* **Beneficios del Aislamiento:** Al ejecutar el receptor dentro de un contenedor, se mitiga el riesgo de que un error en un script de despliegue afecte al sistema host. Las características del kernel de Linux, como los *namespaces* y *cgroups*, aseguran que el proceso de procesamiento de webhooks esté confinado a sus propios recursos.20  
* **Orquestación en Kubernetes:** Los webhooks de GitHub se integran frecuentemente con controladores de Kubernetes para automatizar el despliegue de aplicaciones mediante flujos de GitOps, donde una notificación de GitHub dispara el refresco de las imágenes de contenedor en el clúster.23

## **Configurabilidad avanzada y optimización del rendimiento**

La potencia de los webhooks reside en su capacidad para ser ajustados a las necesidades específicas de cada flujo de trabajo. Esta configurabilidad abarca desde la selección de eventos hasta la topología de red utilizada para la entrega.3

### **Filtrado y gestión de eventos masivos**

GitHub permite una suscripción granular a más de 70 tipos de eventos diferentes.2 Sin embargo, eventos como push pueden generar un volumen masivo de datos si se empujan muchas etiquetas o ramas simultáneamente. Es importante recordar que las cargas útiles están limitadas a 25 MB; si se excede este límite, la entrega no se realizará.8

Para optimizar el rendimiento, los desarrolladores deben aplicar filtros tanto en GitHub como en su servidor 2:

* **Suscripción Mínima:** Solo suscribirse a los eventos estrictamente necesarios para cumplir con el propósito de la integración.3  
* **Filtros de Acción:** Muchos eventos tienen una clave action (ej. opened, labeled, synchronize). El servidor debe verificar esta clave antes de iniciar cualquier procesamiento pesado.2  
* **Monitoreo de Throttling:** En casos de ráfagas extremas de tráfico, GitHub puede aplicar un estrangulamiento (*throttling*) temporal de las entregas. Los registros de entregas fallidas mostrarán una propiedad throttled\_at cuando esto ocurra.27

### **Entrega a sistemas privados y redes restringidas**

Uno de los mayores desafíos técnicos es entregar webhooks a servidores que no tienen una IP pública, como los servidores de CI internos en una red corporativa.26

| Estrategia de Entrega | Mecanismo Técnico | Casos de Uso |
| :---- | :---- | :---- |
| **Proxy Inverso (DMZ)** | Un servidor Nginx expuesto recibe la petición y la reenvía por la red interna.25 | Configuración estándar empresarial. |
| **Túneles (ngrok/zrok)** | Crea un túnel cifrado persistente desde el servidor interno a un punto de enlace público.4 | Desarrollo y pruebas rápidas. |
| **Smee.io / Webhook.site** | Servicios de retransmisión basados en WebSockets que actúan como intermediarios.2 | Entornos de desarrollo local. |
| **Redes Overlay (OpenZiti)** | Redes virtuales privadas que permiten la comunicación directa sin exposición a internet.31 | Alta seguridad y cumplimiento. |

## **Seguridad y mitigación de riesgos en arquitecturas de webhooks**

La exposición de un endpoint público para recibir datos de GitHub introduce vectores de ataque que deben ser mitigados mediante una postura de seguridad multicapa.2

### **Verificación criptográfica de la fuente**

El uso del secreto del webhook y la cabecera X-Hub-Signature-256 es el control de seguridad más importante.7 El servidor receptor debe calcular el HMAC-SHA256 utilizando el secreto configurado y los bytes brutos (*raw body*) de la solicitud recibida. Es fundamental usar los bytes exactos antes de cualquier parseo JSON, ya que incluso un cambio en un espacio en blanco invalidará la firma.7

Para prevenir ataques de comparación, se deben utilizar funciones de igualdad de tiempo constante. Esto evita que un atacante deduzca información sobre el secreto midiendo cuánto tiempo tarda el servidor en rechazar una firma incorrecta.2

### **Protección contra ataques de repetición y suplantación**

Incluso con firmas válidas, un atacante podría interceptar una solicitud legítima y volver a enviarla más tarde para forzar una acción repetida (ataque de repetición).5

* **Validación de X-GitHub-Delivery:** Almacenar los IDs de entrega procesados y rechazar cualquier ID duplicado.5  
* **Listas de IP Permitidas:** Configurar el firewall o el proxy inverso para aceptar solicitudes POST únicamente desde los rangos de IP oficiales de GitHub obtenidos mediante el endpoint /meta.5  
* **Rotación de Secretos:** Al igual que las contraseñas, los secretos de los webhooks deben rotarse periódicamente para limitar el impacto en caso de una filtración accidental.2

### **Conclusiones sobre la operatividad y perspectivas futuras**

Los webhooks de GitHub representan la convergencia entre la simplicidad del protocolo HTTP y la potencia de las arquitecturas dirigidas por eventos. Aunque el servicio carece de un costo directo, su implementación exitosa exige una inversión en infraestructura robusta, preferiblemente basada en Linux o contenedores, para manejar las estrictas restricciones de tiempo de respuesta de 10 segundos y la gestión de la fiabilidad ante la ausencia de reintentos automáticos nativos.5

La tendencia futura apunta hacia una mayor integración con redes privadas y la transición hacia IPv6, lo que permitirá una conectividad más directa y segura entre la nube de GitHub y los centros de datos corporativos.1 En última instancia, la madurez de una integración con GitHub se mide por su capacidad para procesar eventos de forma asíncrona, validar firmas criptográficas con rigor y gestionar la idempotencia mediante los identificadores de entrega únicos, garantizando así un sistema de automatización estable y escalable para el ciclo de vida del desarrollo de software.2

#### **Fuentes citadas**

1. About webhooks \- GitHub Docs, acceso: febrero 24, 2026, [https://docs.github.com/en/webhooks/about-webhooks](https://docs.github.com/en/webhooks/about-webhooks)  
2. GitHub Webhooks: Complete Guide with Event Examples, acceso: febrero 24, 2026, [https://www.magicbell.com/blog/github-webhooks-guide](https://www.magicbell.com/blog/github-webhooks-guide)  
3. Creating webhooks \- GitHub Docs, acceso: febrero 24, 2026, [https://docs.github.com/en/webhooks/using-webhooks/creating-webhooks](https://docs.github.com/en/webhooks/using-webhooks/creating-webhooks)  
4. Handling webhook deliveries \- GitHub Docs, acceso: febrero 24, 2026, [https://docs.github.com/en/webhooks/using-webhooks/handling-webhook-deliveries](https://docs.github.com/en/webhooks/using-webhooks/handling-webhook-deliveries)  
5. Best practices for using webhooks \- GitHub Docs, acceso: febrero 24, 2026, [https://docs.github.com/en/webhooks/using-webhooks/best-practices-for-using-webhooks](https://docs.github.com/en/webhooks/using-webhooks/best-practices-for-using-webhooks)  
6. How can I efficiently handle GitHub webhook retries and avoid duplicate event processing? · community · Discussion \#175725, acceso: febrero 24, 2026, [https://github.com/orgs/community/discussions/175725](https://github.com/orgs/community/discussions/175725)  
7. Validating webhook deliveries \- GitHub Docs, acceso: febrero 24, 2026, [https://docs.github.com/en/webhooks/using-webhooks/validating-webhook-deliveries](https://docs.github.com/en/webhooks/using-webhooks/validating-webhook-deliveries)  
8. Webhook events and payloads \- GitHub Docs, acceso: febrero 24, 2026, [https://docs.github.com/en/webhooks/webhook-events-and-payloads](https://docs.github.com/en/webhooks/webhook-events-and-payloads)  
9. "Signature is Invalid" message for AppCenter Webhook request \- Stack Overflow, acceso: febrero 24, 2026, [https://stackoverflow.com/questions/54921963/signature-is-invalid-message-for-appcenter-webhook-request](https://stackoverflow.com/questions/54921963/signature-is-invalid-message-for-appcenter-webhook-request)  
10. Best practice for securely validating GitHub webhook payloads in a REST API service · community · Discussion \#182735, acceso: febrero 24, 2026, [https://github.com/orgs/community/discussions/182735](https://github.com/orgs/community/discussions/182735)  
11. Types of webhooks \- GitHub Docs, acceso: febrero 24, 2026, [https://docs.github.com/en/webhooks/types-of-webhooks](https://docs.github.com/en/webhooks/types-of-webhooks)  
12. Redelivering webhooks \- GitHub Docs, acceso: febrero 24, 2026, [https://docs.github.com/en/webhooks/testing-and-troubleshooting-webhooks/redelivering-webhooks](https://docs.github.com/en/webhooks/testing-and-troubleshooting-webhooks/redelivering-webhooks)  
13. Pricing Calculator \- GitHub, acceso: febrero 24, 2026, [https://github.com/pricing/calculator](https://github.com/pricing/calculator)  
14. Webhooks documentation \- GitHub Docs, acceso: febrero 24, 2026, [https://docs.github.com/en/webhooks](https://docs.github.com/en/webhooks)  
15. GitHub's plans \- GitHub Enterprise Server 3.18 Docs, acceso: febrero 24, 2026, [https://docs.github.com/en/enterprise-server@3.18/get-started/learning-about-github/githubs-plans](https://docs.github.com/en/enterprise-server@3.18/get-started/learning-about-github/githubs-plans)  
16. Pricing · Plans for every developer \- GitHub, acceso: febrero 24, 2026, [https://github.com/pricing](https://github.com/pricing)  
17. hookdeck/outpost: Open Source Outbound Webhooks and Event Destinations Infrastructure \- GitHub, acceso: febrero 24, 2026, [https://github.com/hookdeck/outpost](https://github.com/hookdeck/outpost)  
18. Rate Limit Cheatsheet for Self-Hosting Github Runners \- WarpBuild Blog, acceso: febrero 24, 2026, [https://www.warpbuild.com/blog/rate-limits-self-hosted-runners](https://www.warpbuild.com/blog/rate-limits-self-hosted-runners)  
19. linuxserver-labs/docker-webhook \- GitHub, acceso: febrero 24, 2026, [https://github.com/linuxserver-labs/docker-webhook](https://github.com/linuxserver-labs/docker-webhook)  
20. learning-knative/README.md at master \- GitHub, acceso: febrero 24, 2026, [https://github.com/danbev/learning-knative/blob/master/README.md](https://github.com/danbev/learning-knative/blob/master/README.md)  
21. Billing and usage \- GitHub Docs, acceso: febrero 24, 2026, [https://docs.github.com/en/actions/concepts/billing-and-usage](https://docs.github.com/en/actions/concepts/billing-and-usage)  
22. Actions limits \- GitHub Docs, acceso: febrero 24, 2026, [https://docs.github.com/en/actions/reference/limits](https://docs.github.com/en/actions/reference/limits)  
23. GitHub Webhook Receiver \- Kargo Docs, acceso: febrero 24, 2026, [https://docs.kargo.io/user-guide/reference-docs/webhook-receivers/github/](https://docs.kargo.io/user-guide/reference-docs/webhook-receivers/github/)  
24. GitHub Webhook Integration | CodeSignal Learn, acceso: febrero 24, 2026, [https://codesignal.com/learn/courses/web-application-and-api/lessons/github-webhook-integration](https://codesignal.com/learn/courses/web-application-and-api/lessons/github-webhook-integration)  
25. adnanh/webhook: webhook is a lightweight incoming webhook server to run shell commands \- GitHub, acceso: febrero 24, 2026, [https://github.com/adnanh/webhook](https://github.com/adnanh/webhook)  
26. Build and deploy locally using GitHub actions and Webhooks | The awesome garage, acceso: febrero 24, 2026, [https://theawesomegarage.com/blog/build-and-deploy-locally-using-github-actions-and-webhooks](https://theawesomegarage.com/blog/build-and-deploy-locally-using-github-actions-and-webhooks)  
27. Troubleshooting webhooks \- GitHub Docs, acceso: febrero 24, 2026, [https://docs.github.com/en/webhooks/testing-and-troubleshooting-webhooks/troubleshooting-webhooks](https://docs.github.com/en/webhooks/testing-and-troubleshooting-webhooks/troubleshooting-webhooks)  
28. Handling failed webhook deliveries \- GitHub Docs, acceso: febrero 24, 2026, [https://docs.github.com/en/webhooks/using-webhooks/handling-failed-webhook-deliveries](https://docs.github.com/en/webhooks/using-webhooks/handling-failed-webhook-deliveries)  
29. Automatically redelivering failed deliveries for a repository webhook \- GitHub Docs, acceso: febrero 24, 2026, [https://docs.github.com/en/webhooks/using-webhooks/automatically-redelivering-failed-deliveries-for-a-repository-webhook](https://docs.github.com/en/webhooks/using-webhooks/automatically-redelivering-failed-deliveries-for-a-repository-webhook)  
30. About GitHub's IP addresses \- GitHub Docs, acceso: febrero 24, 2026, [https://docs.github.com/en/authentication/keeping-your-account-and-data-secure/about-githubs-ip-addresses](https://docs.github.com/en/authentication/keeping-your-account-and-data-secure/about-githubs-ip-addresses)  
31. Delivering webhooks to private systems \- GitHub Docs, acceso: febrero 24, 2026, [https://docs.github.com/en/webhooks/using-webhooks/delivering-webhooks-to-private-systems](https://docs.github.com/en/webhooks/using-webhooks/delivering-webhooks-to-private-systems)  
32. REST API endpoints for rate limits \- GitHub Docs, acceso: febrero 24, 2026, [https://docs.github.com/en/rest/rate-limit/rate-limit](https://docs.github.com/en/rest/rate-limit/rate-limit)  
33. fukamachi/github-webhook: Docker container to listen for GitHub webhook events \- GitHub, acceso: febrero 24, 2026, [https://github.com/fukamachi/github-webhook](https://github.com/fukamachi/github-webhook)  
34. amtp-protocol/agentry: Orchestration and memory for multi-agent systems \- GitHub, acceso: febrero 24, 2026, [https://github.com/amtp-protocol/agentry](https://github.com/amtp-protocol/agentry)  
35. Chapter 10\. Simplified event routing | Using automation decisions \- Red Hat Documentation, acceso: febrero 24, 2026, [https://docs.redhat.com/en/documentation/red\_hat\_ansible\_automation\_platform/2.5/html/using\_automation\_decisions/simplified-event-routing](https://docs.redhat.com/en/documentation/red_hat_ansible_automation_platform/2.5/html/using_automation_decisions/simplified-event-routing)