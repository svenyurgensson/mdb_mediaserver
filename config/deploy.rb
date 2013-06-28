set :user, 'pubapi'    # Username in the server to SSH to.
set :project_name, 'mediaserver'
set :domain,      '54.235.213.159'
set :deploy_to,   "/home/#{user}/#{project_name}"


# Put any custom mkdir's in here for when `mina setup` is ran.
# For Rails apps, we'll make some of the shared paths that are shared between
# all releases.
task :setup => :environment do
  queue! %[mkdir -p "#{deploy_to}"]
  queue! %[chmod g+rx,u+rwx "#{deploy_to}"]

  queue! %[mkdir -p "#{deploy_to}/shared"]
  queue! %[chmod g+rx,u+rwx "#{deploy_to}/shared"]
end

desc "Deploys the current version to the server."
task :deploy => :environment do
  deploy do
    invoke :'exchange:replace_mserv'

    to :launch do
      invoke :'server:restart'
    end
  end
end

def mserv_name
  (`cd build/ && find *-linux` || raise).strip
end

set :service_name, mserv_name

namespace :exchange do
  desc "Delete remote mserv"
  task :delete_mserv do
    queue! echo_cmd("rm -f #{deploy_to}/mserv*-linux")
  end

  desc "Copy local build to remote server"
  task :copy_mserv do
    puts "copying #{service_name} to remote server..."
    `rsync -avz build/#{service_name} #{user}@#{domain}:#{deploy_to}`
  end

  desc "Replace old mserv with new from local build"
  task :replace_mserv do
    invoke :'server:stop'
    invoke :'exchange:delete_mserv'
    invoke :'exchange:copy_mserv'
  end
end

namespace :server do
  desc "Stop mediaserver"
  task :stop do
    queue! echo_cmd('pkill -SIGINT -f mserv 2>/dev/null')
  end

  desc "Start mediaserver"
  task :start do
    queue! echo_cmd("nohup #{deploy_to}/#{service_name} /etc/mserv/mserv.config.yaml 2>/dev/null &")
  end

  desc "Restart mediaserver"
  task :restart do
    invoke :'server:stop'
    invoke :'server:start'
  end
end
