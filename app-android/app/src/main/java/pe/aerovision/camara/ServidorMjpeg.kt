package pe.aerovision.camara

import java.io.BufferedOutputStream
import java.io.BufferedReader
import java.io.IOException
import java.io.InputStreamReader
import java.io.OutputStream
import java.net.InetSocketAddress
import java.net.ServerSocket
import java.net.Socket
import java.net.URI
import java.net.URLDecoder
import java.security.MessageDigest
import java.util.concurrent.ExecutorService
import java.util.concurrent.Executors
import java.util.concurrent.TimeUnit
import java.util.concurrent.atomic.AtomicInteger
import java.util.concurrent.atomic.AtomicLong
import java.util.concurrent.locks.ReentrantLock
import kotlin.concurrent.withLock

/** Servidor HTTP mínimo que transmite el último JPEG publicado como MJPEG (multipart/x-mixed-replace). */
class ServidorMjpeg(private val puerto: Int, @Volatile var token: String) {

    private val cerrojo = ReentrantLock()
    private val hayFrame = cerrojo.newCondition()
    private var frame: ByteArray? = null
    private var secuencia = 0L
    private val clientesActivos = AtomicInteger(0)
    private val framesEnviados = AtomicLong(0)
    private var hilos: ExecutorService? = null
    @Volatile private var socket: ServerSocket? = null

    val clientes: Int get() = clientesActivos.get()
    val enviados: Long get() = framesEnviados.get()
    val activo: Boolean get() = socket != null
    val puertoLocal: Int get() = socket?.localPort ?: -1

    /** Abre el puerto y empieza a aceptar conexiones. */
    fun iniciar() {
        if (socket != null) return
        val servidor = ServerSocket()
        servidor.reuseAddress = true
        servidor.bind(InetSocketAddress(puerto))
        val pool = Executors.newCachedThreadPool()
        socket = servidor
        hilos = pool
        pool.execute { aceptar(servidor, pool) }
    }

    /** Cierra el puerto y corta a todos los clientes. */
    fun detener() {
        val servidor = socket ?: return
        socket = null
        runCatching { servidor.close() }
        cerrojo.withLock { hayFrame.signalAll() }
        hilos?.shutdownNow()
        hilos = null
    }

    /** Deja un JPEG nuevo para todos los clientes conectados. */
    fun publicar(jpeg: ByteArray) {
        cerrojo.withLock {
            frame = jpeg
            secuencia++
            hayFrame.signalAll()
        }
    }

    private fun aceptar(servidor: ServerSocket, pool: ExecutorService) {
        while (!servidor.isClosed) {
            val cliente = try {
                servidor.accept()
            } catch (e: IOException) {
                break
            }
            runCatching { pool.execute { atender(cliente) } }.onFailure { cliente.close() }
        }
    }

    private fun atender(cliente: Socket) {
        cliente.use { conexion ->
            try {
                conexion.soTimeout = 10_000
                conexion.tcpNoDelay = true
                val lector = BufferedReader(InputStreamReader(conexion.getInputStream(), Charsets.ISO_8859_1))
                val peticion = lector.readLine() ?: return
                while (true) {
                    val cabecera = lector.readLine() ?: break
                    if (cabecera.isEmpty()) break
                }
                val partes = peticion.split(" ")
                val salida = BufferedOutputStream(conexion.getOutputStream(), 64 * 1024)
                if (partes.size < 2 || partes[0] != "GET") {
                    responder(salida, "405 Method Not Allowed", "text/plain; charset=utf-8", "Solo GET".toByteArray())
                    return
                }
                val uri = runCatching { URI(partes[1]) }.getOrNull()
                val ruta = uri?.rawPath ?: "/"
                val consulta = parametros(uri?.rawQuery)
                if (ruta == "/") {
                    responder(salida, "200 OK", "text/plain; charset=utf-8",
                        "Camara ESAN: GET /video?token=... (MJPEG), /foto.jpg?token=..., /estado?token=...".toByteArray())
                    return
                }
                if (!tokenValido(consulta["token"])) {
                    responder(salida, "401 Unauthorized", "text/plain; charset=utf-8", "Token invalido".toByteArray())
                    return
                }
                when (ruta) {
                    "/video" -> transmitir(salida)
                    "/foto.jpg" -> {
                        val jpeg = cerrojo.withLock { frame }
                        if (jpeg == null) responder(salida, "503 Service Unavailable", "text/plain; charset=utf-8", "Sin imagen".toByteArray())
                        else responder(salida, "200 OK", "image/jpeg", jpeg)
                    }
                    "/estado" -> responder(salida, "200 OK", "application/json",
                        """{"clientes":$clientes,"enviados":$enviados,"transmitiendo":${frame != null}}""".toByteArray())
                    else -> responder(salida, "404 Not Found", "text/plain; charset=utf-8", "No existe".toByteArray())
                }
            } catch (_: IOException) {
            }
        }
    }

    private fun transmitir(salida: OutputStream) {
        salida.write(("HTTP/1.1 200 OK\r\n" +
            "Content-Type: multipart/x-mixed-replace; boundary=$LIMITE\r\n" +
            "Cache-Control: no-cache, no-store, must-revalidate\r\n" +
            "Pragma: no-cache\r\n" +
            "Connection: close\r\n\r\n").toByteArray())
        salida.flush()
        clientesActivos.incrementAndGet()
        try {
            var visto = -1L
            while (socket != null) {
                val jpeg = cerrojo.withLock {
                    while (secuencia == visto && socket != null) hayFrame.await(1, TimeUnit.SECONDS)
                    visto = secuencia
                    frame
                } ?: continue
                salida.write(("--$LIMITE\r\nContent-Type: image/jpeg\r\nContent-Length: ${jpeg.size}\r\n\r\n").toByteArray())
                salida.write(jpeg)
                salida.write("\r\n".toByteArray())
                salida.flush()
                framesEnviados.incrementAndGet()
            }
        } catch (_: IOException) {
        } catch (_: InterruptedException) {
        } finally {
            clientesActivos.decrementAndGet()
        }
    }

    private fun tokenValido(recibido: String?): Boolean {
        val esperado = token
        if (esperado.isEmpty()) return true
        if (recibido == null) return false
        return MessageDigest.isEqual(esperado.toByteArray(), recibido.toByteArray())
    }

    private fun responder(salida: OutputStream, estado: String, tipo: String, cuerpo: ByteArray) {
        salida.write(("HTTP/1.1 $estado\r\nContent-Type: $tipo\r\nContent-Length: ${cuerpo.size}\r\n" +
            "Cache-Control: no-store\r\nConnection: close\r\n\r\n").toByteArray())
        salida.write(cuerpo)
        salida.flush()
    }

    companion object {
        const val LIMITE = "frameesan"

        /** Parámetros de una query string (?a=1&b=2). */
        fun parametros(query: String?): Map<String, String> =
            query.orEmpty().split("&").filter { it.contains("=") }.associate {
                val (clave, valor) = it.split("=", limit = 2)
                URLDecoder.decode(clave, "UTF-8") to URLDecoder.decode(valor, "UTF-8")
            }
    }
}
