import SwiftUI

struct CameraView: View {
    @Environment(AppState.self) private var appState
    @Environment(\.dismiss) private var dismiss

    let slot: SessionSlot
    let session: Session

    // Recording state
    @State private var isRecording = false
    @State private var recordedDuration: TimeInterval = 0
    @State private var phase: RecordPhase = .idle
    @State private var timer: Timer? = nil
    @State private var pulse = false

    private var maxDuration: TimeInterval { TimeInterval(session.maxSectionDurationSeconds) }

    enum RecordPhase {
        case idle, recording, preview
    }

    var body: some View {
        ZStack {
            Color.black.ignoresSafeArea()

            VStack(spacing: 0) {
                // Header
                HStack {
                    Button { dismiss() } label: {
                        Image(systemName: "xmark")
                            .font(.system(size: 14, weight: .semibold))
                            .foregroundStyle(.white)
                            .frame(width: 36, height: 36)
                            .background(Color.white.opacity(0.15))
                            .clipShape(Circle())
                    }
                    Spacer()
                    Text(slot.name)
                        .font(.system(size: 15, weight: .semibold))
                        .foregroundStyle(.white)
                    Spacer()
                    Color.clear.frame(width: 36, height: 36)
                }
                .padding(.horizontal, Tokens.Spacing.base)
                .padding(.top, Tokens.Spacing.base)

                Spacer()

                // Camera preview area (simulated)
                ZStack {
                    RoundedRectangle(cornerRadius: Tokens.Radius.lg)
                        .fill(Color(white: 0.12))
                        .overlay(
                            RoundedRectangle(cornerRadius: Tokens.Radius.lg)
                                .stroke(Color.white.opacity(0.1), lineWidth: 1)
                        )

                    if phase == .preview {
                        // Simulated preview frame
                        VStack(spacing: Tokens.Spacing.md) {
                            Image(systemName: "checkmark.circle.fill")
                                .font(.system(size: 48))
                                .foregroundStyle(Tokens.Colors.primary)
                            Text("Clip recorded")
                                .font(.system(size: 18, weight: .semibold))
                                .foregroundStyle(.white)
                            Text("\(Int(recordedDuration))s · \(slot.name)")
                                .font(Tokens.Typography.bodySmall)
                                .foregroundStyle(Color.white.opacity(0.6))
                        }
                    } else {
                        // Camera placeholder
                        VStack(spacing: Tokens.Spacing.md) {
                            Image(systemName: "camera.fill")
                                .font(.system(size: 40))
                                .foregroundStyle(Color.white.opacity(0.2))
                            Text("Camera preview")
                                .font(Tokens.Typography.bodySmall)
                                .foregroundStyle(Color.white.opacity(0.3))
                        }
                    }

                    // Recording indicator
                    if phase == .recording {
                        VStack {
                            HStack {
                                Spacer()
                                HStack(spacing: 6) {
                                    Circle()
                                        .fill(Color.red)
                                        .frame(width: 8, height: 8)
                                        .opacity(pulse ? 1 : 0.3)
                                        .animation(.easeInOut(duration: 0.6).repeatForever(), value: pulse)
                                    Text("REC")
                                        .font(.system(size: 11, weight: .bold))
                                        .foregroundStyle(.white)
                                }
                                .padding(.horizontal, 10)
                                .padding(.vertical, 5)
                                .background(Color.red.opacity(0.85))
                                .clipShape(Capsule())
                                .padding()
                            }
                            Spacer()
                        }
                    }

                    // Duration display
                    if phase != .idle {
                        VStack {
                            Spacer()
                            HStack {
                                Spacer()
                                Text(durationText)
                                    .font(.system(size: 13, weight: .semibold, design: .monospaced))
                                    .foregroundStyle(.white)
                                    .padding(.horizontal, 10)
                                    .padding(.vertical, 4)
                                    .background(Color.black.opacity(0.5))
                                    .clipShape(Capsule())
                                    .padding()
                            }
                        }
                    }
                }
                .aspectRatio(9/16, contentMode: .fit)
                .padding(.horizontal, Tokens.Spacing.base)

                Spacer()

                // Controls
                if phase == .preview {
                    previewControls
                } else {
                    recordControls
                }
            }
        }
        .navigationBarHidden(true)
        .onDisappear { stopTimer() }
    }

    // MARK: - Record Controls

    private var recordControls: some View {
        VStack(spacing: Tokens.Spacing.md) {
            Text(phase == .recording ? "Release to stop" : "Hold to record")
                .font(.system(size: 13, weight: .medium))
                .foregroundStyle(Color.white.opacity(0.6))

            Circle()
                .fill(phase == .recording ? Color.red : Tokens.Colors.primary)
                .frame(width: 72, height: 72)
                .overlay(
                    Circle()
                        .stroke(Color.white.opacity(0.3), lineWidth: 3)
                        .frame(width: 82, height: 82)
                )
                .scaleEffect(phase == .recording ? 1.1 : 1.0)
                .animation(.easeInOut(duration: 0.2), value: phase == .recording)
                .gesture(
                    DragGesture(minimumDistance: 0)
                        .onChanged { _ in
                            if phase == .idle { startRecording() }
                        }
                        .onEnded { _ in
                            if phase == .recording { stopRecording() }
                        }
                )

            // Max duration progress
            VStack(spacing: 4) {
                VProgressBar(
                    progress: maxDuration > 0 ? recordedDuration / maxDuration : 0,
                    height: 4
                )
                .padding(.horizontal, Tokens.Spacing.xxxl)
                Text("Max \(Int(maxDuration))s")
                    .font(Tokens.Typography.caption)
                    .foregroundStyle(Color.white.opacity(0.4))
            }
        }
        .padding(.bottom, Tokens.Spacing.xxxl)
    }

    // MARK: - Preview Controls

    private var previewControls: some View {
        HStack(spacing: Tokens.Spacing.md) {
            VButton(title: "Retake", variant: .secondary, size: .lg, isFullWidth: true) {
                retake()
            }
            VButton(title: "Confirm", variant: .primary, size: .lg, isFullWidth: true) {
                confirmClip()
            }
        }
        .padding(.horizontal, Tokens.Spacing.base)
        .padding(.bottom, Tokens.Spacing.xxxl)
    }

    // MARK: - Actions

    private func startRecording() {
        phase = .recording
        pulse = true
        recordedDuration = 0

        timer = Timer.scheduledTimer(withTimeInterval: 0.1, repeats: true) { _ in
            recordedDuration += 0.1
            if recordedDuration >= maxDuration {
                stopRecording()
            }
        }
    }

    private func stopRecording() {
        stopTimer()
        phase = .preview
        pulse = false
    }

    private func stopTimer() {
        timer?.invalidate()
        timer = nil
    }

    private func retake() {
        recordedDuration = 0
        phase = .idle
    }

    private func confirmClip() {
        appState.showToast("Clip saved!")
        dismiss()
    }

    private var durationText: String {
        let secs = Int(recordedDuration)
        let tenths = Int((recordedDuration * 10).truncatingRemainder(dividingBy: 10))
        return "\(secs).\(tenths)s"
    }
}

#Preview {
    CameraView(slot: MockData.slots[0], session: MockData.sessions[0])
        .environment(AppState())
}
