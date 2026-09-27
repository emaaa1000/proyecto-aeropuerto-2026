package pe.aerovision.camara

import android.Manifest
import android.content.ClipData
import android.content.ClipboardManager
import android.content.pm.PackageManager
import android.graphics.Bitmap
import android.graphics.Matrix
import android.os.Bundle
import android.os.Handler
import android.os.Looper
import android.os.SystemClock
import android.util.Size
import android.view.WindowManager
import android.widget.Button
import android.widget.TextView
import android.widget.Toast
import androidx.activity.result.contract.ActivityResultContracts
import androidx.appcompat.app.AppCompatActivity
import androidx.camera.core.CameraSelector
import androidx.camera.core.ImageAnalysis
import androidx.camera.core.ImageProxy
import androidx.camera.core.Preview
import androidx.camera.core.resolutionselector.ResolutionSelector
import androidx.camera.core.resolutionselector.ResolutionStrategy
import androidx.camera.lifecycle.ProcessCameraProvider
import androidx.camera.view.PreviewView
import androidx.core.content.ContextCompat
import java.io.ByteArrayOutputStream
import java.net.Inet4Address
import java.net.NetworkInterface
import java.security.SecureRandom
import java.util.concurrent.ExecutorService
import java.util.concurrent.Executors

/** Convierte el teléfono en una cámara IP: vista previa, servidor MJPEG con token y URLs para conectarse. */
class MainActivity : AppCompatActivity() {

    private lateinit var vista: PreviewView
    private lateinit var estado: TextView
    private lateinit var urls: TextView
    private lateinit var botonTransmitir: Button
    private lateinit var botonCamara: Button
    private lateinit var botonCopiar: Button
    private lateinit var servidor: ServidorMjpeg
    private lateinit var analisis: ExecutorService
    private val principal = Handler(Looper.getMainLooper())
    private var lente = CameraSelector.LENS_FACING_BACK
    @Volatile private var transmitiendo = false
    private var ultimoEnvio = 0L
    private var enviadosAntes = 0L

    private val pedirPermiso = registerForActivityResult(ActivityResultContracts.RequestPermission()) { concedido ->
        if (concedido) iniciarCamara() else estado.text = getString(R.string.sin_permiso)
    }

    private val refresco = object : Runnable {
        override fun run() {
            actualizarEstado()
            principal.postDelayed(this, 1000)
        }
    }

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        setContentView(R.layout.activity_main)
        window.addFlags(WindowManager.LayoutParams.FLAG_KEEP_SCREEN_ON)
        vista = findViewById(R.id.vista)
        estado = findViewById(R.id.estado)
        urls = findViewById(R.id.urls)
        botonTransmitir = findViewById(R.id.boton_transmitir)
        botonCamara = findViewById(R.id.boton_camara)
        botonCopiar = findViewById(R.id.boton_copiar)
        analisis = Executors.newSingleThreadExecutor()
        servidor = ServidorMjpeg(PUERTO, tokenGuardado())

        botonTransmitir.setOnClickListener { if (transmitiendo) detenerTransmision() else iniciarTransmision() }
        botonCamara.setOnClickListener {
            lente = if (lente == CameraSelector.LENS_FACING_BACK) CameraSelector.LENS_FACING_FRONT else CameraSelector.LENS_FACING_BACK
            iniciarCamara()
        }
        botonCopiar.setOnClickListener { copiarUrl() }

        if (ContextCompat.checkSelfPermission(this, Manifest.permission.CAMERA) == PackageManager.PERMISSION_GRANTED) iniciarCamara()
        else pedirPermiso.launch(Manifest.permission.CAMERA)
        actualizarEstado()
    }

    override fun onResume() {
        super.onResume()
        principal.post(refresco)
    }

    override fun onPause() {
        super.onPause()
        principal.removeCallbacks(refresco)
    }

    override fun onDestroy() {
        super.onDestroy()
        servidor.detener()
        analisis.shutdown()
    }

    /** Token aleatorio que se genera una vez y queda guardado en el teléfono. */
    private fun tokenGuardado(): String {
        val preferencias = getSharedPreferences("camara", MODE_PRIVATE)
        preferencias.getString("token", null)?.let { return it }
        val alfabeto = "abcdefghijkmnpqrstuvwxyz23456789"
        val aleatorio = SecureRandom()
        val token = (1..10).map { alfabeto[aleatorio.nextInt(alfabeto.length)] }.joinToString("")
        preferencias.edit().putString("token", token).apply()
        return token
    }

    /** Vista previa y análisis de imagen (solo se codifica a JPEG cuando alguien está mirando). */
    private fun iniciarCamara() {
        val futuro = ProcessCameraProvider.getInstance(this)
        futuro.addListener({
            val proveedor = futuro.get()
            val preview = Preview.Builder().build()
            preview.setSurfaceProvider(vista.surfaceProvider)
            val resolucion = ResolutionSelector.Builder()
                .setResolutionStrategy(ResolutionStrategy(Size(1280, 720), ResolutionStrategy.FALLBACK_RULE_CLOSEST_LOWER_THEN_HIGHER))
                .build()
            val captura = ImageAnalysis.Builder()
                .setResolutionSelector(resolucion)
                .setBackpressureStrategy(ImageAnalysis.STRATEGY_KEEP_ONLY_LATEST)
                .setOutputImageFormat(ImageAnalysis.OUTPUT_IMAGE_FORMAT_RGBA_8888)
                .build()
            captura.setAnalyzer(analisis) { imagen -> codificar(imagen) }
            val selector = CameraSelector.Builder().requireLensFacing(lente).build()
            try {
                proveedor.unbindAll()
                proveedor.bindToLifecycle(this, selector, preview, captura)
            } catch (e: Exception) {
                estado.text = getString(R.string.error_camara, e.message ?: "")
            }
        }, ContextCompat.getMainExecutor(this))
    }

    private fun codificar(imagen: ImageProxy) {
        imagen.use {
            if (!transmitiendo || servidor.clientes == 0) return
            val ahora = SystemClock.elapsedRealtime()
            if (ahora - ultimoEnvio < INTERVALO_MS) return
            ultimoEnvio = ahora
            val original = it.toBitmap()
            val rotacion = it.imageInfo.rotationDegrees
            val derecha = if (rotacion == 0) original else Bitmap.createBitmap(
                original, 0, 0, original.width, original.height, Matrix().apply { postRotate(rotacion.toFloat()) }, true)
            val jpeg = ByteArrayOutputStream(96 * 1024)
            derecha.compress(Bitmap.CompressFormat.JPEG, CALIDAD_JPEG, jpeg)
            servidor.publicar(jpeg.toByteArray())
        }
    }

    private fun iniciarTransmision() {
        try {
            servidor.iniciar()
            transmitiendo = true
        } catch (e: Exception) {
            Toast.makeText(this, getString(R.string.error_puerto, PUERTO, e.message ?: ""), Toast.LENGTH_LONG).show()
        }
        actualizarEstado()
    }

    private fun detenerTransmision() {
        transmitiendo = false
        servidor.detener()
        actualizarEstado()
    }

    private fun direcciones(): List<String> =
        runCatching {
            NetworkInterface.getNetworkInterfaces().toList()
                .filter { it.isUp && !it.isLoopback }
                .flatMap { it.inetAddresses.toList() }
                .filterIsInstance<Inet4Address>()
                .map { it.hostAddress ?: "" }
                .filter { it.isNotEmpty() }
        }.getOrDefault(emptyList())

    private fun urlVideo(ip: String) = "http://$ip:$PUERTO/video?token=${servidor.token}"

    private fun copiarUrl() {
        val ip = direcciones().firstOrNull()
        if (ip == null) {
            Toast.makeText(this, R.string.sin_red, Toast.LENGTH_SHORT).show()
            return
        }
        val portapapeles = getSystemService(ClipboardManager::class.java)
        portapapeles.setPrimaryClip(ClipData.newPlainText("URL de la cámara", urlVideo(ip)))
        Toast.makeText(this, R.string.url_copiada, Toast.LENGTH_SHORT).show()
    }

    private fun actualizarEstado() {
        val ips = direcciones()
        urls.text = if (ips.isEmpty()) getString(R.string.sin_red) else ips.joinToString("\n") { urlVideo(it) }
        val enviados = servidor.enviados
        val fps = enviados - enviadosAntes
        enviadosAntes = enviados
        estado.text = if (transmitiendo)
            getString(R.string.estado_transmitiendo, servidor.clientes, fps, servidor.token)
        else getString(R.string.estado_detenido)
        botonTransmitir.setText(if (transmitiendo) R.string.detener else R.string.transmitir)
    }

    companion object {
        const val PUERTO = 8080
        const val INTERVALO_MS = 66L
        const val CALIDAD_JPEG = 70
    }
}
