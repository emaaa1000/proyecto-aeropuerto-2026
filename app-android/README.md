# Cámara ESAN (Android)

Convierte el teléfono en una **cámara IP**: transmite su cámara como MJPEG por HTTP para que el modelo final (LAP01) la procese en la laptop y la web muestre el tracking en vivo (**Teléfonos**). Se pueden conectar varios teléfonos a la vez y nada de lo procesado se guarda.

```
Teléfono (esta app)  ──MJPEG──▶  Modelo/Test Modelo/camara_telefono.py (GPU)  ──WebSocket──────▶  web
http://IP:8080/video?token=…      YOLO26m · tracker · Re-ID · género                                 Teléfonos (solo memoria)
```

## Instalar

- Desde la web: **Teléfonos → ⬇ CamaraESAN.apk** (el teléfono debe abrir la web: `https://<IP-de-la-laptop>:8443/telefonos`).
- O por cable: `adb install app/build/outputs/apk/debug/app-debug.apk`.

Android pedirá permitir «instalar apps de origen desconocido» para el navegador. Requiere Android 7.0 o superior.

## Usar

1. Abre **Cámara ESAN**, concede el permiso de cámara y pulsa **Transmitir**.
2. La app muestra la URL con su token, por ejemplo `http://192.168.1.50:8080/video?token=ab3k9x2m4q`. **Copiar URL** la pone en el portapapeles.
3. Pégala en la web: **Teléfonos → ＋ Agregar teléfono** (uno por cada teléfono).
4. En la laptop: `python "Modelo/Test Modelo/camara_telefono.py"`.

La pantalla queda encendida mientras transmites; al salir de la app la cámara se libera. **Cambiar cámara** alterna trasera/frontal.

## Protocolo

| Ruta | Respuesta |
|---|---|
| `GET /video?token=…` | MJPEG (`multipart/x-mixed-replace; boundary=frameesan`), hasta ~15 fps, JPEG calidad 70, 1280×720 |
| `GET /foto.jpg?token=…` | El último frame |
| `GET /estado?token=…` | `{"clientes": n, "enviados": n, "transmitiendo": bool}` |

Sin el token correcto responde `401`. El token se genera una vez al instalar y queda guardado en el teléfono. Solo se codifica a JPEG cuando hay alguien conectado, para ahorrar batería.

## Red: misma Wi-Fi o IP pública

- **Misma red** (recomendado): teléfono y laptop en la misma Wi-Fi, o la laptop conectada al punto de acceso del teléfono. Funciona directo.
- **Por internet**: el teléfono no tiene IP pública propia (los operadores móviles usan CGNAT). Hace falta un reenvío de puertos en el router hacia el puerto 8080 del teléfono (con la Wi-Fi de casa) o un túnel (por ejemplo Cloudflare Tunnel o ngrok apuntando a `http://IP-del-teléfono:8080`). La URL pública va igual con `?token=…`; el token es lo que evita que cualquiera vea la cámara, no lo compartas.

## Compilar

Requiere JDK 17 y el Android SDK (plataforma 35). Con `ANDROID_HOME` y `JAVA_HOME` definidos:

```bash
./gradlew assembleDebug testDebugUnitTest      # Windows: gradlew.bat
```

El APK queda en `app/build/outputs/apk/debug/app-debug.apk`. Las pruebas (`app/src/test`) levantan el servidor MJPEG en la PC y verifican el token, el formato multipart byte a byte y la foto.

## Probar sin teléfono

`python "Modelo/Test Modelo/simular_telefono.py"` sirve un video del dataset con el mismo protocolo en `http://127.0.0.1:8090/video?token=prueba`.
