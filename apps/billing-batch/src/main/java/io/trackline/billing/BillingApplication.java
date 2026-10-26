package io.trackline.billing;

import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.springframework.batch.core.BatchStatus;
import org.springframework.batch.core.Job;
import org.springframework.batch.core.JobExecution;
import org.springframework.batch.core.JobParameters;
import org.springframework.batch.core.JobParametersBuilder;
import org.springframework.batch.core.launch.JobLauncher;
import org.springframework.boot.ApplicationArguments;
import org.springframework.boot.ApplicationRunner;
import org.springframework.boot.ExitCodeGenerator;
import org.springframework.boot.SpringApplication;
import org.springframework.boot.autoconfigure.SpringBootApplication;
import org.springframework.stereotype.Component;

@SpringBootApplication
public class BillingApplication {
    public static void main(String[] args) {
        // The exit code is the contract with the CronJob: 0 only if the invoices were
        // written and reconciled.
        System.exit(SpringApplication.exit(SpringApplication.run(BillingApplication.class, args)));
    }
}

/**
 * Launches the job once and turns its outcome into the process exit code:
 * 0 = completed and reconciled, 2 = the job failed (including a reconciliation
 * failure),
 * 64 = bad arguments.
 */
@Component
class JobRunner implements ApplicationRunner, ExitCodeGenerator {
    private static final Logger LOG = LoggerFactory.getLogger(JobRunner.class);

    private final JobLauncher launcher;
    private final Job job;
    private int exitCode = 0;

    JobRunner(JobLauncher launcher, Job usageToInvoices) {
        this.launcher = launcher;
        this.job = usageToInvoices;
    }

    @Override
    public void run(ApplicationArguments args) {
        BillingPeriod period;
        try {
            period = resolvePeriod(args);
        } catch (RuntimeException e) {
            LOG.error("invalid period (use --period=YYYY-MM): {}", e.getMessage());
            exitCode = 64;
            return;
        }

        JobParameters params = new JobParametersBuilder().addString("period", period.label())
                .addLong("startedAt", System.currentTimeMillis()).toJobParameters();
        try {
            JobExecution execution = launcher.run(job, params);
            exitCode = execution.getStatus() == BatchStatus.COMPLETED ? 0 : 2;
            if (exitCode != 0) {
                LOG.error("billing for {} did not complete: {}", period.label(), execution.getAllFailureExceptions());
            } else {
                LOG.info("billing for {} completed", period.label());
            }
        } catch (Exception e) {
            LOG.error("billing for {} could not run", period.label(), e);
            exitCode = 2;
        }
    }

    private static BillingPeriod resolvePeriod(ApplicationArguments args) {
        if (args.containsOption("period") && !args.getOptionValues("period").isEmpty()) {
            return BillingPeriod.parse(args.getOptionValues("period").get(0));
        }
        String fromEnv = System.getenv("BILLING_PERIOD");
        if (fromEnv != null && !fromEnv.isBlank()) {
            return BillingPeriod.parse(fromEnv);
        }
        return BillingPeriod.previousMonth();
    }

    @Override
    public int getExitCode() {
        return exitCode;
    }
}
