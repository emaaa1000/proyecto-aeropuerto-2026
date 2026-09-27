package pe.aerovision.camara

import org.junit.After
import org.junit.Assert.assertArrayEquals
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Before
import org.junit.Test
import java.io.BufferedInputStream
import java.io.ByteArrayOutputStream
import java.io.InputStream
import java.net.Socket
import kotlin.concurrent.thread

class ServidorMjpegTest {

    private lateinit var servidor: ServidorMjpeg

    @Before
    fun iniciar() {
        servidor = ServidorMjpeg(0, "secreto")
        servidor.iniciar()
    }

    @After
    fun detener() = servidor.detener()

    private fun conectar(ruta: String): Pair<Socket, BufferedInputStream> {
        val socket = Socket("127.0.0.1", servidor.puertoLocal)
        socket.soTimeout = 5000
        socket.getOutputStream().write("GET $ruta HTTP/1.1\r\nHost: prueba\r\n\r\n".toByteArray())
        return socket to BufferedInputStream(socket.getInputStream())
    }

    private fun linea(entrada: InputStream): String {
        val bytes = ByteArrayOutputStream()
        while (true) {
            val b = entrada.read()
            if (b < 0 || b == '\n'.code) break
            if (b != '\r'.code) bytes.write(b)
        }
        return bytes.toString(Charsets.ISO_8859_1.name())
    }

    private fun cabeceras(entrada: InputStream): Map<String, String> {
        val resultado = mutableMapOf<String, String>()
        while (true) {
            val l = linea(entrada)
            if (l.isEmpty()) return resultado
            val (clave, valor) = l.split(":", limit = 2)
            resultado[clave.trim().lowercase()] = valor.trim()
        }
    }

    @Test
    fun rechazaSinToken() {
        val (socket, entrada) = conectar("/video")
        socket.use { assertTrue(linea(entrada).contains("401")) }
        val (otro, entrada2) = conectar("/video?token=otro")
        otro.use { assertTrue(linea(entrada2).contains("401")) }
    }

    @Test
    fun transmiteCadaFrameCompletoComoMjpeg() {
        val frames = (1..5).map { n -> ByteArray(3000 + n * 17) { (it * n).toByte() } }
        val (socket, entrada) = conectar("/video?token=secreto")
        socket.use {
            assertTrue(linea(entrada).contains("200"))
            val tipo = cabeceras(entrada)["content-type"].orEmpty()
            assertTrue(tipo.startsWith("multipart/x-mixed-replace"))
            thread {
                while (servidor.clientes == 0) Thread.sleep(5)
                for (f in frames) {
                    servidor.publicar(f)
                    Thread.sleep(80)
                }
            }
            for (esperado in frames) {
                assertEquals("--${ServidorMjpeg.LIMITE}", linea(entrada))
                val parte = cabeceras(entrada)
                assertEquals("image/jpeg", parte["content-type"])
                val largo = parte["content-length"]!!.toInt()
                val datos = ByteArray(largo)
                var leido = 0
                while (leido < largo) leido += entrada.read(datos, leido, largo - leido)
                assertArrayEquals(esperado, datos)
                linea(entrada)
            }
            assertEquals(1, servidor.clientes)
        }
    }

    @Test
    fun fotoDevuelveElUltimoFrame() {
        val jpeg = byteArrayOf(1, 2, 3, 4, 5)
        servidor.publicar(jpeg)
        val (socket, entrada) = conectar("/foto.jpg?token=secreto")
        socket.use {
            assertTrue(linea(entrada).contains("200"))
            val largo = cabeceras(entrada)["content-length"]!!.toInt()
            val datos = ByteArray(largo)
            var leido = 0
            while (leido < largo) leido += entrada.read(datos, leido, largo - leido)
            assertArrayEquals(jpeg, datos)
        }
    }

    @Test
    fun leeParametrosDeLaUrl() {
        assertEquals(mapOf("token" to "a b", "x" to "1"), ServidorMjpeg.parametros("token=a%20b&x=1"))
        assertEquals(emptyMap<String, String>(), ServidorMjpeg.parametros(null))
    }
}
