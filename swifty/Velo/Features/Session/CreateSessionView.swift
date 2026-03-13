import SwiftUI

struct CreateSessionView: View {
    @Environment(AppState.self) private var appState
    @Environment(\.dismiss) private var dismiss

    // Step
    @State private var step = 1

    // Step 1 fields
    @State private var sessionName = ""
    @State private var selectedDuration = 15        // seconds
    @State private var selectedSlotIndices = Set<Int>()
    @State private var deadline = Date().addingTimeInterval(8 * 3600)

    // Step 2
    @State private var createdSession: Session? = nil
    @State private var copied = false

    private let durations = MockData.sectionDurationOptions
    private let presets = MockData.presetSlots

    private var isStep1Valid: Bool {
        !selectedSlotIndices.isEmpty
    }

    private var inviteURL: String {
        "https://velo.app/join/\(createdSession?.id.prefix(8) ?? "xxxxxxxx")"
    }

    var body: some View {
        ZStack {
            Tokens.Colors.background.ignoresSafeArea()

            VStack(spacing: 0) {
                // Navigation header
                HStack {
                    Button { dismiss() } label: {
                        Image(systemName: "xmark")
                            .font(.system(size: 14, weight: .semibold))
                            .foregroundStyle(Tokens.Colors.mutedForeground)
                            .frame(width: 36, height: 36)
                            .background(Tokens.Colors.muted)
                            .clipShape(Circle())
                    }
                    Spacer()
                    // Progress pills
                    HStack(spacing: 6) {
                        ForEach(1...2, id: \.self) { i in
                            Capsule()
                                .fill(i <= step ? Tokens.Colors.primary : Tokens.Colors.border)
                                .frame(width: i == step ? 24 : 16, height: 4)
                        }
                    }
                    Spacer()
                    Color.clear.frame(width: 36, height: 36)
                }
                .padding(.horizontal, Tokens.Spacing.base)
                .padding(.vertical, Tokens.Spacing.md)

                ScrollView {
                    if step == 1 {
                        step1Content
                    } else {
                        step2Content
                    }
                }
            }
        }
        .navigationBarHidden(true)
    }

    // MARK: - Step 1

    private var step1Content: some View {
        VStack(alignment: .leading, spacing: Tokens.Spacing.xl) {
            VStack(alignment: .leading, spacing: Tokens.Spacing.sm) {
                Text("New Session")
                    .font(.system(size: 28, weight: .bold))
                    .foregroundStyle(Tokens.Colors.foreground)
                Text("Configure your session settings.")
                    .font(Tokens.Typography.bodyBase)
                    .foregroundStyle(Tokens.Colors.mutedForeground)
            }

            // Session name
            VInput(
                label: "Session Name",
                placeholder: "e.g. Sunday Vibes",
                text: $sessionName,
                maxLength: 40
            )

            // Duration picker
            VStack(alignment: .leading, spacing: Tokens.Spacing.sm) {
                Text("Max Clip Duration")
                    .font(Tokens.Typography.label)
                    .foregroundStyle(Tokens.Colors.foreground)
                HStack(spacing: Tokens.Spacing.sm) {
                    ForEach(durations, id: \.self) { d in
                        Button {
                            selectedDuration = d
                        } label: {
                            Text("\(d)s")
                                .font(.system(size: 14, weight: .semibold))
                                .foregroundStyle(selectedDuration == d ? Tokens.Colors.primaryForeground : Tokens.Colors.foreground)
                                .frame(maxWidth: .infinity)
                                .frame(height: 40)
                                .background(selectedDuration == d ? Tokens.Colors.primary : Tokens.Colors.muted)
                                .clipShape(RoundedRectangle(cornerRadius: Tokens.Radius.md))
                        }
                        .buttonStyle(.plain)
                    }
                }
            }

            // Time slots
            VStack(alignment: .leading, spacing: Tokens.Spacing.sm) {
                HStack {
                    Text("Time Slots")
                        .font(Tokens.Typography.label)
                        .foregroundStyle(Tokens.Colors.foreground)
                    Spacer()
                    Text("\(selectedSlotIndices.count) selected")
                        .font(Tokens.Typography.caption)
                        .foregroundStyle(Tokens.Colors.mutedForeground)
                }

                VStack(spacing: Tokens.Spacing.sm) {
                    ForEach(presets.indices, id: \.self) { i in
                        let preset = presets[i]
                        let selected = selectedSlotIndices.contains(i)
                        Button {
                            if selected { selectedSlotIndices.remove(i) }
                            else { selectedSlotIndices.insert(i) }
                        } label: {
                            HStack {
                                Image(systemName: selected ? "checkmark.circle.fill" : "circle")
                                    .foregroundStyle(selected ? Tokens.Colors.primary : Tokens.Colors.border)
                                    .font(.system(size: 18))
                                Text(preset.name)
                                    .font(Tokens.Typography.bodyBase)
                                    .fontWeight(.medium)
                                    .foregroundStyle(Tokens.Colors.foreground)
                                Spacer()
                                Text("\(preset.start) – \(preset.end)")
                                    .font(Tokens.Typography.bodySmall)
                                    .foregroundStyle(Tokens.Colors.mutedForeground)
                            }
                            .padding(Tokens.Spacing.md)
                            .background(selected ? Tokens.Colors.primaryLight : Tokens.Colors.muted)
                            .clipShape(RoundedRectangle(cornerRadius: Tokens.Radius.md))
                            .overlay(
                                RoundedRectangle(cornerRadius: Tokens.Radius.md)
                                    .stroke(selected ? Tokens.Colors.primary.opacity(0.4) : .clear, lineWidth: 1)
                            )
                        }
                        .buttonStyle(.plain)
                    }
                }
            }

            // Deadline picker
            VStack(alignment: .leading, spacing: Tokens.Spacing.sm) {
                Text("Session Deadline")
                    .font(Tokens.Typography.label)
                    .foregroundStyle(Tokens.Colors.foreground)
                DatePicker(
                    "",
                    selection: $deadline,
                    in: Date().addingTimeInterval(3600)...,
                    displayedComponents: [.date, .hourAndMinute]
                )
                .labelsHidden()
                .padding(Tokens.Spacing.md)
                .background(Tokens.Colors.muted)
                .clipShape(RoundedRectangle(cornerRadius: Tokens.Radius.md))
            }

            VButton(
                title: "Continue",
                variant: .primary,
                size: .lg,
                isFullWidth: true,
                isDisabled: !isStep1Valid
            ) {
                createSession()
                step = 2
            }
            .padding(.bottom, Tokens.Spacing.xxxl)
        }
        .padding(.horizontal, Tokens.Spacing.base)
    }

    // MARK: - Step 2

    private var step2Content: some View {
        VStack(alignment: .leading, spacing: Tokens.Spacing.xl) {
            VStack(alignment: .leading, spacing: Tokens.Spacing.sm) {
                Image(systemName: "checkmark.circle.fill")
                    .font(.system(size: 40))
                    .foregroundStyle(Tokens.Colors.primary)
                Text("Session Created!")
                    .font(.system(size: 28, weight: .bold))
                    .foregroundStyle(Tokens.Colors.foreground)
                Text("Invite friends with the link below. The link is valid until your session deadline.")
                    .font(Tokens.Typography.bodyBase)
                    .foregroundStyle(Tokens.Colors.mutedForeground)
            }

            // Summary card
            if let session = createdSession {
                VCard {
                    VStack(alignment: .leading, spacing: Tokens.Spacing.md) {
                        sessionDetailRow(label: "Session", value: session.name)
                        Divider()
                        sessionDetailRow(label: "Slots", value: "\(session.slots.count) time slots")
                        Divider()
                        sessionDetailRow(label: "Max clip", value: "\(session.maxSectionDurationSeconds)s per slot")
                        Divider()
                        sessionDetailRow(label: "Deadline", value: formatDeadline(session.deadline))
                    }
                }
            }

            // Invite link
            VStack(alignment: .leading, spacing: Tokens.Spacing.sm) {
                Text("Invite Link")
                    .font(Tokens.Typography.label)
                    .foregroundStyle(Tokens.Colors.foreground)

                HStack {
                    Text(inviteURL)
                        .font(.system(size: 13, design: .monospaced))
                        .foregroundStyle(Tokens.Colors.mutedForeground)
                        .lineLimit(1)
                        .truncationMode(.middle)
                    Spacer()
                    Button {
                        UIPasteboard.general.string = inviteURL
                        copied = true
                        Task {
                            try? await Task.sleep(for: .seconds(2))
                            await MainActor.run { copied = false }
                        }
                    } label: {
                        HStack(spacing: 4) {
                            Image(systemName: copied ? "checkmark" : "doc.on.doc")
                                .font(.system(size: 12, weight: .medium))
                            Text(copied ? "Copied!" : "Copy")
                                .font(.system(size: 12, weight: .semibold))
                        }
                        .foregroundStyle(copied ? Tokens.Colors.success : Tokens.Colors.primary)
                        .padding(.horizontal, 12)
                        .padding(.vertical, 6)
                        .background(copied ? Tokens.Colors.successLight : Tokens.Colors.primaryLight)
                        .clipShape(Capsule())
                    }
                    .buttonStyle(.plain)
                }
                .padding(Tokens.Spacing.md)
                .background(Tokens.Colors.muted)
                .clipShape(RoundedRectangle(cornerRadius: Tokens.Radius.md))
            }

            VButton(
                title: "Start Recording",
                variant: .primary,
                size: .lg,
                isFullWidth: true
            ) {
                dismiss()
            }
            .padding(.bottom, Tokens.Spacing.xxxl)
        }
        .padding(.horizontal, Tokens.Spacing.base)
    }

    // MARK: - Helpers

    private func sessionDetailRow(label: String, value: String) -> some View {
        HStack {
            Text(label)
                .font(Tokens.Typography.bodySmall)
                .foregroundStyle(Tokens.Colors.mutedForeground)
            Spacer()
            Text(value)
                .font(Tokens.Typography.bodySmall)
                .fontWeight(.medium)
                .foregroundStyle(Tokens.Colors.foreground)
        }
    }

    private func formatDeadline(_ date: Date) -> String {
        let formatter = DateFormatter()
        formatter.dateFormat = "MMM d, h:mm a"
        return formatter.string(from: date)
    }

    private func createSession() {
        let name = sessionName.trimmingCharacters(in: .whitespaces).isEmpty
            ? "Session — \(DateFormatter.sessionDefault.string(from: Date()))"
            : sessionName

        let sortedIndices = selectedSlotIndices.sorted()
        let slots: [SessionSlot] = sortedIndices.enumerated().map { idx, i in
            let p = presets[i]
            return SessionSlot(
                id: "slot-\(UUID().uuidString.prefix(8))",
                name: p.name,
                startsAt: p.start,
                endsAt: p.end,
                slotOrder: idx,
                status: nil
            )
        }

        let session = Session(
            id: UUID().uuidString,
            creatorId: appState.currentUser?.id ?? "",
            name: name,
            status: .active,
            deadline: deadline,
            sectionCount: slots.count,
            maxSectionDurationSeconds: selectedDuration,
            slots: slots,
            participants: [
                Participant(
                    id: UUID().uuidString,
                    userId: appState.currentUser?.id ?? "",
                    displayName: appState.currentUser?.displayName ?? "",
                    joinedAt: Date(),
                    status: .active
                )
            ],
            clips: [],
            reelUrl: nil,
            completedAt: nil,
            createdAt: Date()
        )

        createdSession = session
        appState.createSession(session)
    }
}

extension DateFormatter {
    static let sessionDefault: DateFormatter = {
        let f = DateFormatter()
        f.dateFormat = "MMM d"
        return f
    }()
}

#Preview {
    NavigationStack {
        CreateSessionView()
    }
    .environment(AppState())
}
